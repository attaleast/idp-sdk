package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

type Publisher interface {
	Publish(ctx context.Context, subject string, event any) error
	Close()
}

type Bus struct {
	nc *nats.Conn
	js nats.JetStreamContext
}

func NewNATS(url string) (*Bus, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connecto to NATS: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to get JetStream context: %w", err)
	}

	_, err = js.StreamInfo("EVENTS")
	if err != nil {
		_, err := js.AddStream(&nats.StreamConfig{
			Name:     "EVENTS",
			Subjects: []string{">"},
			Storage:  nats.FileStorage,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create EVENTS stream: %w", err)
		}
	}

	return &Bus{nc: nc, js: js}, nil
}

func (b *Bus) Publish(ctx context.Context, subject string, event any) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	_, err = b.js.Publish(subject, data)
	return err
}

func (b *Bus) Close() {
	b.nc.Drain()
}
