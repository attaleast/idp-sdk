package messaging

import (
	"fmt"

	"github.com/nats-io/nats.go"
)

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

	return &Bus{nc: nc, js: js}, nil
}

func (b *Bus) Publish(subject string, data []byte) error {
	_, err := b.js.Publish(subject, data)
	return err
}

func (b *Bus) Close() {
	b.nc.Drain()
}
