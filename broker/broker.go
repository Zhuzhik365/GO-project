package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	QueueAdCreated       = "ads.created"
	QueueAdStatusChanged = "ads.status.changed"
)

type AdCreatedMessage struct {
	AdID      int64     `json:"ad_id"`
	UserID    int64     `json:"user_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type AdStatusChangedMessage struct {
	AdID      int64     `json:"ad_id"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Rabbit struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func New(url string) (*Rabbit, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect rabbitmq: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}

	r := &Rabbit{conn: conn, ch: ch}
	if err := r.declare(); err != nil {
		_ = r.Close()
		return nil, err
	}
	return r, nil
}

func (r *Rabbit) Close() error {
	if r.ch != nil {
		_ = r.ch.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

func (r *Rabbit) declare() error {
	queues := []string{QueueAdCreated, QueueAdStatusChanged}
	for _, q := range queues {
		if _, err := r.ch.QueueDeclare(q, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare queue %s: %w", q, err)
		}
	}
	return nil
}

func (r *Rabbit) PublishAdCreated(ctx context.Context, msg AdCreatedMessage) error {
	return r.publish(ctx, QueueAdCreated, msg)
}

func (r *Rabbit) PublishAdStatusChanged(ctx context.Context, msg AdStatusChangedMessage) error {
	return r.publish(ctx, QueueAdStatusChanged, msg)
}

func (r *Rabbit) publish(ctx context.Context, queue string, msg any) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	return r.ch.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	})
}

func (r *Rabbit) ConsumeAdCreated(ctx context.Context, handler func(context.Context, AdCreatedMessage) error) error {
	return r.consume(ctx, QueueAdCreated, func(ctx context.Context, body []byte) error {
		var msg AdCreatedMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("unmarshal ad created: %w", err)
		}
		return handler(ctx, msg)
	})
}

func (r *Rabbit) ConsumeAdStatusChanged(ctx context.Context, handler func(context.Context, AdStatusChangedMessage) error) error {
	return r.consume(ctx, QueueAdStatusChanged, func(ctx context.Context, body []byte) error {
		var msg AdStatusChangedMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("unmarshal status changed: %w", err)
		}
		return handler(ctx, msg)
	})
}

func (r *Rabbit) consume(ctx context.Context, queue string, handler func(context.Context, []byte) error) error {
	msgs, err := r.ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume queue %s: %w", queue, err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				return nil
			}
			if err := handler(ctx, msg.Body); err != nil {
				_ = msg.Nack(false, true)
				continue
			}
			_ = msg.Ack(false)
		}
	}
}
