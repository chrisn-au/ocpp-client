package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/lorenzodonini/ocpp-go/transport"
	"github.com/lorenzodonini/ocpp-go/transport/redis"

	"ocpp-server/internal/server"
)

const (
	defaultRedisAddr = "localhost:6379"
	defaultHTTPPort  = "8081"
	defaultMQTTHost  = "localhost"
	defaultMQTTPort  = 1883
)

func main() {
	// Parse environment variables
	redisAddr := getEnvOrDefault("REDIS_ADDR", defaultRedisAddr)
	redisPassword := os.Getenv("REDIS_PASSWORD")
	httpPort := getEnvOrDefault("HTTP_PORT", defaultHTTPPort)

	// Parse MQTT configuration
	mqttEnabled := getEnvOrDefault("MQTT_ENABLED", "false") == "true"
	mqttHost := getEnvOrDefault("MQTT_HOST", defaultMQTTHost)
	mqttPortStr := getEnvOrDefault("MQTT_PORT", strconv.Itoa(defaultMQTTPort))
	mqttPort, err := strconv.Atoi(mqttPortStr)
	if err != nil {
		log.Fatalf("Invalid MQTT_PORT: %v", err)
	}
	mqttUsername := os.Getenv("MQTT_USERNAME")
	mqttPassword := os.Getenv("MQTT_PASSWORD")
	mqttClientID := getEnvOrDefault("MQTT_CLIENT_ID", "ocpp-server")
	mqttBusinessEventsEnabled := getEnvOrDefault("MQTT_BUSINESS_EVENTS_ENABLED", "true") == "true"

	// Parse Redis state TTL
	stateTTLStr := getEnvOrDefault("REDIS_STATE_TTL", "10m")
	stateTTL, err := time.ParseDuration(stateTTLStr)
	if err != nil {
		log.Fatalf("Invalid REDIS_STATE_TTL: %v", err)
	}

	// Create Redis transport configuration
	config := &transport.RedisConfig{
		Addr:                redisAddr,
		Password:            redisPassword,
		DB:                  0,
		ChannelPrefix:       "ocpp",
		UseDistributedState: true,
		StateKeyPrefix:      "ocpp",
		StateTTL:            stateTTL,
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
			RedisAddr:                 redisAddr,
			RedisPassword:             redisPassword,
			HTTPPort:                  httpPort,
			MQTTEnabled:               mqttEnabled,
			MQTTHost:                  mqttHost,
			MQTTPort:                  mqttPort,
			MQTTUsername:              mqttUsername,
			MQTTPassword:              mqttPassword,
			MQTTClientID:              mqttClientID,
			MQTTBusinessEventsEnabled: mqttBusinessEventsEnabled,
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