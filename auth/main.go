package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"github.com/sakashimaa/billing-microservice/auth/config"
	"github.com/sakashimaa/billing-microservice/auth/lib/workers"
	"github.com/sakashimaa/billing-microservice/auth/repository"
	redis2 "github.com/sakashimaa/billing-microservice/auth/repository/redis"
	"github.com/sakashimaa/billing-microservice/auth/services"
	"github.com/sakashimaa/billing-microservice/auth/tokens"
	"github.com/sakashimaa/billing-microservice/contracts/gen/auth_pb"
	"github.com/sakashimaa/billing-microservice/pkg/broker"
	"github.com/sakashimaa/billing-microservice/pkg/infrastructure/interceptors"
	"github.com/sakashimaa/billing-microservice/pkg/outbox"
	"github.com/sakashimaa/billing-microservice/pkg/utils/env"
	"github.com/sakashimaa/billing-microservice/pkg/utils/storage"
	"google.golang.org/grpc"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := storage.NewPool(context.Background(), env.ParseEnvWithFallback("DATABASE_URL", ""))
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}
	defer db.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisUrl,
	})
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Fatalf("failed to close redis: %v", err)
		}
	}()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("failed to ping redis: %v", err)
	}

	conn, err := amqp.Dial(cfg.BrokerUrl)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Fatalf("failed to close rabbitmq connection: %v", err)
		}
	}()

	outboxRepo := outbox.NewPgxRepository(db)
	authRepo := repository.NewAuthRepository(db)
	cacheRepo := redis2.NewTokenCache(rdb)
	token := tokens.NewJWTManager(cfg.Auth)
	authService := services.NewAuthService(authRepo, outboxRepo, token, cfg.Auth, cacheRepo)
	grpcHandler := NewGRPCHandler(authService)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("failed to declare channel: %v", err)
	}
	defer func() {
		if err := ch.Close(); err != nil {
			log.Fatalf("failed to close channel: %v", err)
		}
	}()

	exchangeName := "auth.events"
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
		log.Fatalf("failed to exchange declare: %v", err)
	}

	rabbitPublisher := broker.NewRabbitPublisher(ch, exchangeName)
	outboxWorker := outbox.NewOutboxWorker(outboxRepo, rabbitPublisher)
	go outboxWorker.StartOutboxWorker(ctx)

	limit := env.ParseEnvWithFallback("REFRESH_WORKER_LIMIT", 5000)
	go workers.StartCleanupWorker(ctx, authRepo, limit)

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(interceptors.AuthInterceptor()),
	)
	auth_pb.RegisterAuthServiceServer(grpcServer, grpcHandler)

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to start grpc on 50052")
	}

	errCh := make(chan error, 1)
	go func() {
		log.Println("server is listening on 50052")
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("OS signal received, initiating graceful shutdown...")

		grpcServer.GracefulStop()
		log.Println("Server stopped properly")
	case err := <-errCh:
		log.Fatalf("gRPC server crashed: %v", err)
	}
}
