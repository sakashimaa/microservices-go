package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sakashimaa/billing-microservice/billing/lib/broker"
	"github.com/sakashimaa/billing-microservice/billing/lib/workers"
	"github.com/sakashimaa/billing-microservice/billing/repository"
	"github.com/sakashimaa/billing-microservice/billing/services"
	"github.com/sakashimaa/billing-microservice/contracts/gen/billing_pb"
	"github.com/sakashimaa/billing-microservice/pkg/infrastructure/interceptors"
	"github.com/sakashimaa/billing-microservice/pkg/utils/env"
	"github.com/sakashimaa/billing-microservice/pkg/utils/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("failed to load .env: %v", err)
	}

	dbUrl := env.ParseEnvWithFallback("DATABASE_URL", "")
	if dbUrl == "" {
		log.Fatalf("db connection string is not set in .env")
	}

	pool, err := storage.NewPool(context.Background(), dbUrl)
	if err != nil {
		log.Fatalf("failed to initialize pool: %v", err)
	}

	conn, err := amqp.Dial(env.ParseEnvWithFallback("BROKER_URL", ""))
	if err != nil {
		log.Fatalf("failed to connect to RabbitMQ")
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Fatalf("failed to close connection to RabbitMQ")
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	billingRepo := repository.NewBillingRepo(pool)
	svc := services.NewBillingService(pool, billingRepo)
	billingAdapter := NewGRPCServer(svc)

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("failed to open a channel: %v\n", err)
	}
	defer func() {
		if err := ch.Close(); err != nil {
			log.Fatalf("failed to close channel: %v\n", err)
		}
	}()

	exchangeName := "billing.events"
	err = ch.ExchangeDeclare(
		exchangeName,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("failed to declare an exchange: %v\n", err)
	}

	rabbitPublisher := broker.NewRabbitPublisher(ch, exchangeName)

	outboxWorker := workers.NewOutboxWorker(ctx, billingRepo, rabbitPublisher)
	go outboxWorker.StartOutboxWorker()

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(interceptors.AuthInterceptor()),
	)
	billing_pb.RegisterBillingServiceServer(grpcServer, billingAdapter)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen on port 50051: %v\n", err)
	}

	errCh := make(chan error, 1)
	go func() {
		log.Println("Server is listening on :50051")
		if err = grpcServer.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("OS shutdown signal received, initiating graceful shutdown...")

		grpcServer.GracefulStop()
		log.Println("server stopped successfully")
	case err := <-errCh:
		log.Fatalf("failed to start: %v", err)
	}
}
