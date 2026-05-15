package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sakashimaa/billing-microservice/contracts/gen/auth_pb"
	"github.com/sakashimaa/billing-microservice/notification/internal/config"
	"github.com/sakashimaa/billing-microservice/notification/internal/repository"
	"github.com/sakashimaa/billing-microservice/notification/internal/service"
	"github.com/sakashimaa/billing-microservice/notification/internal/worker"
	"github.com/sakashimaa/billing-microservice/pkg/utils/env"
	"github.com/sakashimaa/billing-microservice/pkg/utils/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/gomail.v2"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.NewConfig("./.env")
	if err != nil {
		log.Fatalf("failed to read config: %v", err)
	}

	dbConn := env.ParseEnvWithFallback("DATABASE_URL", "")

	pool, err := storage.NewPool(context.Background(), dbConn)
	if err != nil {
		log.Fatalf("failed to create db pool: %v", err)
	}
	defer pool.Close()

	conn, err := amqp.Dial(cfg.BrokerUrl)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Fatalf("failed to close rabbitmq: %v", err)
		}
	}()

	authAddr := env.ParseEnvWithFallback("AUTH_ADDR", "localhost:50052")
	authConn, err := grpc.NewClient(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to auth via gRPC: %v", err)
	}
	defer func(authConn *grpc.ClientConn) {
		if err := authConn.Close(); err != nil {
			log.Fatalf("failed to close auth conn: %v", err)
		}
	}(authConn)

	dialer := gomail.NewDialer(
		cfg.SmtpHost,
		cfg.SmtpPort,
		cfg.SmtpEmail,
		cfg.SmtpPass,
	)

	authClient := auth_pb.NewAuthServiceClient(authConn)
	notificationService := service.NewNotificationService(dialer)

	notificationRepo := repository.NewInboxRepository(pool)

	consumer := worker.NewRabbitConsumer(conn, notificationService, authClient, notificationRepo, pool)

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, os.Interrupt)
	defer stop()

	var wg sync.WaitGroup

	errCh := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- consumer.StartConsume(
			shutdownCtx,
			"notification.events",
			"#",
			"billing.events",
			"auth.events",
		)
	}()

	log.Println("notification service is running...")
	select {
	case err := <-errCh:
		if err != nil {
			log.Fatalf("consumer crashed: %v", err)
		}
	case <-shutdownCtx.Done():
		log.Println("received shutdown signal, waiting for workers to finish...")

		wg.Wait()
		log.Println("All workers are dropped. Cleaning up resources...")
	}
}
