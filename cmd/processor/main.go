package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-project/broker"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rabbitURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	rabbit, err := openRabbitWithRetry(ctx, rabbitURL)
	if err != nil {
		log.Fatalf("connect rabbitmq: %v", err)
	}
	defer rabbit.Close()

	processingDelay := getDurationEnv("PROCESSING_DELAY", 3*time.Second)
	log.Println("ad processor service started")

	err = rabbit.ConsumeAdCreated(ctx, func(ctx context.Context, msg broker.AdCreatedMessage) error {
		log.Printf("new ad received: ad_id=%d user_id=%d title=%q", msg.AdID, msg.UserID, msg.Title)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(processingDelay):
		}

		statusMsg := broker.AdStatusChangedMessage{
			AdID:      msg.AdID,
			Status:    "active",
			UpdatedAt: time.Now().UTC(),
		}
		if err := rabbit.PublishAdStatusChanged(ctx, statusMsg); err != nil {
			return err
		}
		log.Printf("status changed event sent: ad_id=%d status=active", msg.AdID)
		return nil
	})
	if err != nil && ctx.Err() == nil {
		log.Fatalf("consume ad created: %v", err)
	}
}

func openRabbitWithRetry(ctx context.Context, url string) (*broker.Rabbit, error) {
	var lastErr error
	for i := 0; i < 30; i++ {
		r, err := broker.New(url)
		if err == nil {
			return r, nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, lastErr
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return d
}
