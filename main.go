package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lorenzodonini/ocpp-go/transport"
	"github.com/lorenzodonini/ocpp-go/transport/redis"

	"ocpp-server/internal/server"
)

const (
	defaultRedisAddr = "localhost:6379"
	defaultHTTPPort  = "8081"
)

func main() {
	// Parse environment variables
	redisAddr := getEnvOrDefault("REDIS_ADDR", defaultRedisAddr)
	redisPassword := os.Getenv("REDIS_PASSWORD")
	httpPort := getEnvOrDefault("HTTP_PORT", defaultHTTPPort)

	// Create Redis transport configuration
	config := &transport.RedisConfig{
		Addr:                redisAddr,
		Password:            redisPassword,
		DB:                  0,
		ChannelPrefix:       "ocpp",
		UseDistributedState: true,
		StateKeyPrefix:      "ocpp",
		StateTTL:            30 * time.Second,
	}

	// Create factory and components
	factory := redis.NewRedisFactory()

	redisTransport, err := factory.CreateTransport(config)
	if err != nil {
		log.Fatalf("Failed to create Redis transport: %v", err)
	}

	serverState, err := factory.CreateServerState(config)
	if err != nil {
		log.Fatalf("Failed to create server state: %v", err)
	}

	businessState, err := factory.CreateBusinessState(config)
	if err != nil {
		log.Fatalf("Failed to create business state: %v", err)
	}

	// Create server
	srv, err := server.NewServer(
		server.Config{
			RedisAddr:     redisAddr,
			RedisPassword: redisPassword,
			HTTPPort:      httpPort,
		},
		redisTransport,
		businessState,
		serverState,
	)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Starting OCPP server with Redis at %s and HTTP API on port %s...", redisAddr, httpPort)

	if err := srv.Start(ctx, config, httpPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// getEnvOrDefault returns environment variable value or default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}