// Package kafka wraps segmentio/kafka-go with sane production defaults:
// a Writer configured for at-least-one delivery with balanced
// partitioning, and cousmer groups (via Reader's GroupID) so multiple
// replicas of a service split partitions automatically instead of each
// replica reprocessing every message
package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// Config configures both NewProducer and NewConsumer. See
// Config.KafkaConfig for the matching env bindings
type Config struct {
	Brokers []string
	GroupID string // required for NewConsumer, ignored by NewProducer
}

// Producer wraps *kafka.Writer for a single topic
type Producer struct {
	writer *kafka.Writer
}

// NewProducer builds a Producer for topic with balanced partitioning
// (round-robin unless a message key is set, in which case same-key
// messages land on the same partition - important for ordering
// guarantees per aggregate/entity ID) and required-acks-all for
// durability
func NewProducer(cfg Config, topic string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(cfg.Brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			Async:        false,
			BatchTimeout: 10 * time.Millisecond,
		},
	}
}

// Publish sends one message. Pass a non-empty key to guarantee ordering
// for messages sharing that key (e.g. an aggregate/entity ID)
func (p *Producer) Publish(ctx context.Context, key, value []byte) error {
	err := p.writer.WriteMessages(ctx, kafka.Message{Key: key, Value: value, Time: time.Now()})
	if err != nil {
		return fmt.Errorf("kafka: publish to %s: %w", p.writer.Topic, err)
	}
	return nil
}

// Check implements health.Checker by verifying the configured brokers
// are reachable
func (p *Producer) Check(ctx context.Context) error {
	conn, err := kafka.DialContext(ctx, "tpc", p.writer.Addr.String())
	if err != nil {
		return fmt.Errorf("kafka: broker unreachable: %w", err)
	}
	return conn.Close()
}

// Close flushes and closes the underlying writer
func (p *Producer) Close() error { return p.writer.Close() }

// Consumer wraps *kafka.Reader bound to a consumer group
type Consumer struct {
	reader *kafka.Reader
}

// NewConsumer builds a Consumer subscribed to topic as part of cfg.GroupID
// - running N replicas of the service with the same GroupID splits the
// topic's partitions across them automatically
func NewConsumer(cfg Config, topic string) (*Consumer, error) {
	if cfg.GroupID == "" {
		return nil, fmt.Errorf("kafka: Config.GroupID is required for consumer")
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          topic,
		GroupID:        cfg.GroupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: 0, // commit synchronously after each processed message
	})
	return &Consumer{reader: reader}, nil
}

// Consume reads messages and calls handler for each; the offset is
// commited only after handler returns nil, so a crash mid-processing
// redelivers the message (at-least-once). Blocks until ctx is cancelled
// or handler returns a non-nil error, which stops the loop and is
// returned to the caller.
func (c *Consumer) Consume(ctx context.Context, handler func(kafka.Message) error) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("kafka: fetch message: %w", err)
		}

		if err := handler(msg); err != nil {
			return fmt.Errorf("kafka: handler error, stopping consume loop: %w", err)
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			return fmt.Errorf("kafka: commit offset: %w", err)
		}
	}
}

// Close closes the reade, leaving the consumer group cleanly
func (c *Consumer) Close() error { return c.reader.Close() }
