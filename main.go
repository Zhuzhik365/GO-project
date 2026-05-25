package main

import (
	"context"
	"database/sql"
	"go-project/handler"
	"go-project/repository"
	"go-project/service"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	dsn := getEnv("POSTGRES_DSN", "postgres://postgres:postgres@127.0.0.1:5432/mini_avito?sslmode=disable")
	jwtSecret := getEnv("JWT_SECRET", "dev-secret-change-me")

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("connect database: %v", err)
	}

	repo := repository.NewTestRepository(db)
	if err := repo.Init(ctx); err != nil {
		log.Fatalf("init database: %v", err)
	}
	defer repo.Close()

	svc := service.NewTestService(repo, jwtSecret)
	h := handler.NewTestHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("/test", h.Test)
	mux.HandleFunc("/dbtest", h.DBTest)
	mux.HandleFunc("/users/register", h.Register)
	mux.HandleFunc("/users/login", h.Login)
	mux.HandleFunc("/register", h.Register)
	mux.HandleFunc("/login", h.Login)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		log.Println("server started on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}

	log.Println("server stopped")
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
