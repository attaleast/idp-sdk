// Package nats wraps nats.go for two most common demo/production
// patterns: fire-and-forget pub/sub (core NATS) and durable, at-least-once
// delivery (JetStream). Core NATS has no persistence - a subscriber that
// isn't connected when a message is published simply never sees it - so
// reach for JetStream (Client.JetStream / PublishJS / SubscribeDurable)
// whenever delivery must survive a subscriber restart.
package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Config configures New. See config.NATSConfig for the matching env
// binding.
type Config struct {
	URL            string
	ConnectName    string        // shows up `nats server list connections`, default "sdk-client"
	ConnectTimeout time.Duration // default 5s
}

// Client wraps *nats.Conn plus an optional JetStream context
type Client struct {
	Conn *nats.Conn
	JS   jetstream.JetStream
}

// New connects to cfg.URL with production-sensible reconnect behavior
// (unlimited reconnect attempts, since a NATS pod restarting is routine
// in k8s and should not be a fatal error for this client)
func New(ctx context.Context, cfg Config) (*Client, error) {
	name := cfg.ConnectName
	if name == "" {
		name = "sdk-client"
	}
	timeout := cfg.ConnectTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	conn, err := nats.Connect(
		cfg.URL,
		nats.Name(name),
		nats.Timeout(timeout),
		nats.MaxReconnects(-1), // retry forever
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("nats: connect to %s: %w", cfg.URL, err)
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("nats: creating jetstream context: %w", err)
	}

	return &Client{Conn: conn, JS: js}, nil
}

// Check implements health.Checker
func (c *Client) Check(ctx context.Context) error {
	if !c.Conn.IsConnected() {
		return fmt.Errorf("nats: not connected (status: %s)", c.Conn.Status())
	}
	return nil
}

// Close drains (finishes in-flight publishes/subscriptions) then closes
// the connection. Prefer this over Conn.Close() directly during
// shutdown so in-flight messages aren't dropped
func (c *Client) Close() error {
	return c.Conn.Drain()
}

// Publish sends a fire-and-forget core NATS message. No delivery
// guarantee beyond "the server accepted it" - use PublishJS for anything
// that must survive a subscriber being offline
func (c *Client) Publish(subject string, data []byte) error {
	return c.Conn.Publish(subject, data)
}

// Subscribe registeres a core NATS handler. Returns an unsubscribe func.
func (c *Client) Subscribe(subject string, handler func(*nats.Msg)) (func() error, error) {
	sub, err := c.Conn.Subscribe(subject, handler)
	if err != nil {
		return nil, fmt.Errorf("nats: subscribe to %s: %w", subject, err)
	}
	return sub.Unsubscribe, nil
}

// EnsureStream creates a JetStream stream if it doesn't already exists -
// idempotent, safe to call on every service startup
func (c *Client) EnsureStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) {
	stream, err := c.JS.Stream(ctx, cfg.Name)
	if err == nil {
		return stream, nil
	}

	stream, err = c.JS.CreateStream(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("nats: creating stream %s: %w", cfg.Name, err)
	}

	return stream, nil
}

// PublishJS publishes to a JetStream-backed subject and waits for the
// server to durably persist it (at-least-once delivery to durable
// consumers, unlike core Publish)
func (c *Client) PublishJS(ctx context.Context, subject string, data []byte) error {
	_, err := c.JS.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("nats: jetstream publish to %s: %w", subject, err)
	}
	return nil
}

// ConsumeDurable creates (or reuses) a durable pull consumer on stream
// and processes messages with handler, acking on success and nak'ing
// (triggering redelivery) on error. Blocks until ctx is cancelled.
func (c *Client) ConsumeDurable(ctx context.Context, streamName, durableName string, handler func(jetstream.Msg) error) error {
	stream, err := c.JS.Stream(ctx, streamName)
	if err != nil {
		return fmt.Errorf("nats: stream %s: %w", streamName, err)
	}
	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:   durableName,
		AckPolicy: jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return fmt.Errorf("nats: creating consumer %s: %w", durableName, err)
	}

	cons, err := consumer.Consume(func(msg jetstream.Msg) {
		if err := handler(msg); err != nil {
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
	if err != nil {
		return fmt.Errorf("nats: starting consume loop: %w", err)
	}
	defer cons.Stop()

	<-ctx.Done()
	return ctx.Err()
}
