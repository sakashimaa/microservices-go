package main

import (
	"context"
	"log"
	"net"

	"github.com/redis/go-redis/v9"
	"github.com/sakashimaa/billing-microservice/auth/config"
	"github.com/sakashimaa/billing-microservice/auth/repository"
	redis2 "github.com/sakashimaa/billing-microservice/auth/repository/redis"
	"github.com/sakashimaa/billing-microservice/auth/services"
	"github.com/sakashimaa/billing-microservice/auth/tokens"
	"github.com/sakashimaa/billing-microservice/contracts/gen/auth_pb"
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

	repo := repository.NewAuthRepository(db)
	cacheRepo := redis2.NewTokenCache(rdb)
	token := tokens.NewJWTManager(cfg.Auth)
	authService := services.NewAuthService(repo, token, cfg.Auth, cacheRepo)
	grpcHandler := NewGRPCHandler(authService)

	grpcServer := grpc.NewServer()
	auth_pb.RegisterAuthServiceServer(grpcServer, grpcHandler)

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to start grpc on 50052")
	}

	log.Println("server is listening on 50052")
	if err = grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve grpc: %v", err)
	}
}
