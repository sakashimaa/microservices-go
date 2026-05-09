package main

import (
	"context"
	"log"
	"net"

	"github.com/sakashimaa/billing-microservice/auth/config"
	"github.com/sakashimaa/billing-microservice/auth/repository"
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

	repo := repository.NewAuthRepository(db)
	token := tokens.NewJWTManager(cfg.Auth)
	authService := services.NewAuthService(repo, token)
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
