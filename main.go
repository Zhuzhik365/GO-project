package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"go-project/broker"
	"go-project/handler"
	"go-project/repository"
	"go-project/service"

	_ "github.com/lib/pq"
)

var totalRequests atomic.Int64

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := getEnv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/mini_avito?sslmode=disable")
	db, err := openDBWithRetry(ctx, dsn)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer db.Close()

	repo := repository.NewPostgresRepository(db)
	if err := repo.Init(ctx); err != nil {
		log.Fatalf("init database: %v", err)
	}

	rabbitURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	rabbit, err := openRabbitWithRetry(ctx, rabbitURL)
	if err != nil {
		log.Fatalf("connect rabbitmq: %v", err)
	}
	defer rabbit.Close()

	go func() {
		log.Println("main service started consuming ad status updates")
		err := rabbit.ConsumeAdStatusChanged(ctx, func(ctx context.Context, msg broker.AdStatusChangedMessage) error {
			log.Printf("received status update: ad_id=%d status=%s", msg.AdID, msg.Status)
			return repo.UpdateAdStatus(ctx, msg.AdID, msg.Status)
		})
		if err != nil && ctx.Err() == nil {
			log.Printf("status consumer stopped: %v", err)
		}
	}()

	svc := service.NewAppService(repo, getEnv("JWT_SECRET", "dev-secret-change-me"), rabbit)
	h := handler.New(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("/test", h.Test)
	mux.HandleFunc("/dbtest", h.DBTest)
	mux.HandleFunc("/users/register", h.Register)
	mux.HandleFunc("/users/login", h.Login)
	mux.HandleFunc("/register", h.Register)
	mux.HandleFunc("/login", h.Login)
	mux.HandleFunc("/ads/create", h.AuthMiddleware(h.CreateAd))
	mux.HandleFunc("/ads", h.AuthMiddleware(h.GetMyAds))
	mux.HandleFunc("/metrics", metricsHandler)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           requestCounter(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Println("app service listening on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen and serve: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	log.Println("server stopped gracefully")
}

func requestCounter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		totalRequests.Add(1)
		next.ServeHTTP(w, r)
	})
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w, "# HELP mini_avito_http_requests_total Total HTTP requests.\n")
	_, _ = fmt.Fprintf(w, "# TYPE mini_avito_http_requests_total counter\n")
	_, _ = fmt.Fprintf(w, "mini_avito_http_requests_total %d\n", totalRequests.Load())
}

func openDBWithRetry(ctx context.Context, dsn string) (*sql.DB, error) {
	var lastErr error
	for i := 0; i < 30; i++ {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			lastErr = err
		} else if err = db.PingContext(ctx); err == nil {
			return db, nil
		} else {
			lastErr = err
			_ = db.Close()
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, lastErr
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
