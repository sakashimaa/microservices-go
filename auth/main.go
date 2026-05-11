package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"
	"github.com/sakashimaa/billing-microservice/auth/config"
	"github.com/sakashimaa/billing-microservice/auth/lib/workers"
	"github.com/sakashimaa/billing-microservice/auth/repository"
	redis2 "github.com/sakashimaa/billing-microservice/auth/repository/redis"
	"github.com/sakashimaa/billing-microservice/auth/services"
	"github.com/sakashimaa/billing-microservice/auth/tokens"
	"github.com/sakashimaa/billing-microservice/contracts/gen/auth_pb"
	"github.com/sakashimaa/billing-microservice/pkg/infrastructure/interceptors"
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

	authRepo := repository.NewAuthRepository(db)
	cacheRepo := redis2.NewTokenCache(rdb)
	token := tokens.NewJWTManager(cfg.Auth)
	authService := services.NewAuthService(authRepo, token, cfg.Auth, cacheRepo)
	grpcHandler := NewGRPCHandler(authService)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
