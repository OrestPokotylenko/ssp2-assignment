package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"southpark-messenger/services/api/adapters/httpadapter"
	"southpark-messenger/services/api/adapters/rabbitmq"
	"southpark-messenger/services/api/internal/app"
)

const (
	defaultHTTPAddress = ":8080"
	defaultRabbitMQURL = "amqp://guest:guest@rabbitmq:5672/"
	defaultQueueName   = "southpark_messages"
)

type config struct {
	httpAddress string
	rabbitMQURL string
	queueName   string
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("API stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg := loadConfig()
	rootContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	publisher, err := connectPublisher(rootContext, logger, cfg.rabbitMQURL, cfg.queueName)
	if err != nil {
		return err
	}
	defer func() {
		if err := publisher.Close(); err != nil {
			logger.Error("could not close RabbitMQ publisher", "error", err)
		}
	}()

	service := app.NewMessageService(publisher)
	handler := httpadapter.NewHandler(service, logger)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:              cfg.httpAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("HTTP API listening", "address", cfg.httpAddress)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-rootContext.Done():
		logger.Info("shutting down HTTP API")
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shut down HTTP server: %w", err)
		}
		return nil
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	}
}

func loadConfig() config {
	return config{
		httpAddress: envOrDefault("HTTP_ADDR", defaultHTTPAddress),
		rabbitMQURL: envOrDefault("RABBITMQ_URL", defaultRabbitMQURL),
		queueName:   envOrDefault("RABBITMQ_QUEUE", defaultQueueName),
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func connectPublisher(ctx context.Context, logger *slog.Logger, url, queue string) (*rabbitmq.Publisher, error) {
	const retryDelay = 2 * time.Second

	for {
		publisher, err := rabbitmq.NewPublisher(url, queue)
		if err == nil {
			logger.Info("connected to RabbitMQ", "queue", queue)
			return publisher, nil
		}

		logger.Warn("RabbitMQ is not ready; retrying", "error", err, "retry_in", retryDelay)
		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return nil, fmt.Errorf("connect to RabbitMQ: %w", ctx.Err())
		case <-timer.C:
		}
	}
}
