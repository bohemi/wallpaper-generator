package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wallpaper-generator/internal/api"
	"wallpaper-generator/internal/database"
)

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func main() {
	// Read ENVs
	pgConnStr := getEnv("DB_URL", "postgres://wallpaper_admin:mysecretpassword@localhost:5432/wallpaper_db?sslmode=disable")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	serverPort := getEnv("PORT", ":8080")
	cacheTTL := 5 * time.Minute

	// 1. Database Connection
	db, err := database.Connect(pgConnStr)
	if err != nil {
		log.Fatalf("PostgreSQL connection failed: %v", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL")

	// 2. Redis Connection
	rdb, err := database.NewRedisClient(redisAddr)
	if err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}
	defer rdb.Close()
	log.Println("Connected to Redis")

	// 3. Router & Server Setup
	srvCore := api.NewServer(db, rdb, cacheTTL)
	httpServer := &http.Server{
		Addr:         serverPort,
		Handler:      srvCore.Routes(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 4. Graceful Shutdown listener
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Server listening on port %s\n", serverPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-shutdownChan
	log.Println("Shutting down server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited cleanly.")
}