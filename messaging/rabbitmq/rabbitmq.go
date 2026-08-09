// Package rabbitmq wraps amqp-091-go with automatic reconnection (
// the stock library gives you a connection and channel that simply die on
// broker restart - every production wrapper needs to handler that) plus a
// small publish/consume API using topic exchange, which covers the
// large majority of routing needs without exposing full AMQP topology
// setup at every call site.
package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Config configuers New. See config.RabbitMQConfig for the matching env
// binding
type Config struct {
	URL            string
	Exchange       string        // topic exchange name, default "events"
	ReconnectDelay time.Duration // default 2s
}

// Client manages a reconnectiong AMQP connection + channel
type Client struct {
	cfg  Config
	mu   sync.RWMutex
	conn *amqp.Connection
	ch   *amqp.Channel

	closeCh chan struct{}
}

// New connects to cfg.URL, declares the topic exchange, and starts a
// background goroutine that reconnects on connection loss until Close is
// called
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Exchange == "" {
		cfg.Exchange = "events"
	}
	if cfg.ReconnectDelay <= 0 {
		cfg.ReconnectDelay = 2 * time.Second
	}

	c := &Client{cfg: cfg, closeCh: make(chan struct{})}
	if err := c.connect(); err != nil {
		return nil, err
	}
	go c.reconnectLoop()
	return c, nil
}

func (c *Client) connect() error {
	conn, err := amqp.Dial(c.cfg.URL)
	if err != nil {
		return fmt.Errorf("rabbitmq: dial %s: %w", redactURL(c.cfg.URL), err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("rabbitmq: open channel: %w", err)
	}
	if err := ch.ExchangeDeclare(c.cfg.Exchange, "topic", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("rabbitmq: declare exchange %s: %w", c.cfg.Exchange, err)
	}

	c.mu.Lock()
	c.conn, c.ch = conn, ch
	c.mu.Unlock()
	return nil
}

func (c *Client) reconnectLoop() {
	for {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		notifyClose := conn.NotifyClose(make(chan *amqp.Error, 1))
		select {
		case <-c.closeCh:
			return
		case err := <-notifyClose:
			if err == nil {
				return // closed intentionally via Close()
			}
		}

		for {
			select {
			case <-c.closeCh:
				return
			case <-time.After(c.cfg.ReconnectDelay):
			}
			if err := c.connect(); err == nil {
				break
			}
		}
	}
}

// Check implements helath.Checker
func (c *Client) Check(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn == nil || c.conn.IsClosed() {
		return fmt.Errorf("rabbitmq: connection closed")
	}
	return nil
}

// Close stops the reconnect loop and closes the channel/connection
func (c *Client) Close() error {
	close(c.closeCh)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Publish sends a persistent message with routingKey on the configured
// topic exchange
func (c *Client) Publish(ctx context.Context, routingKey string, body []byte) error {
	c.mu.RLock()
	ch := c.ch
	c.mu.RUnlock()
	if ch == nil {
		return fmt.Errorf("rabbitmq: not connected")
	}
	return ch.PublishWithContext(ctx, c.cfg.Exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
	})
}

// ConsumeConfig describes the queue a Consume call binds and reads from
type ConsumeConfig struct {
	Queue      string
	RoutingKey string // binding pattern, e.g. "orders.*"
	Durable    bool   // default true
}

func (c *Client) Consume(ctx context.Context, cfg ConsumeConfig, handler func(amqp.Delivery) error) error {
	c.mu.RLock()
	ch := c.ch
	c.mu.RUnlock()
	if ch == nil {
		return fmt.Errorf("rabbitmq: not connected")
	}

	durable := cfg.Durable
	if cfg.Queue == "" {
		durable = false // anonymous queues are inherently transient
	}
	q, err := ch.QueueDeclare(cfg.Queue, durable, !durable, !durable, false, nil)
	if err != nil {
		return fmt.Errorf("rabbitmq: declare queue %s: %w", cfg.Queue, err)
	}
	if err := ch.QueueBind(q.Name, cfg.RoutingKey, c.cfg.Exchange, false, nil); err != nil {
		return fmt.Errorf("rabbitmq: bind queue %s to %s: %w", q.Name, cfg.RoutingKey, err)
	}

	deliveries, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("rabbitmq: consume from %s: %w", q.Name, err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("rabbitmq: delivery channel closed")
			}
			if err := handler(d); err != nil {
				_ = d.Nack(false, true)
				continue
			}
			_ = d.Ack(false)
		}
	}
}

func redactURL(url string) string {
	// amqp URLs embed credentials (amqp://user:pass@host); never log them
	// verbatim
	return "amqp://<redacted>"
}
