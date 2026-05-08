package main

import (
	"context"
	"log"
	"net"

	"github.com/joho/godotenv"
	"github.com/sakashimaa/billing-microservice/billing/services"
	"github.com/sakashimaa/billing-microservice/contracts/gen/billing_pb"
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

	svc := services.NewBillingPGService(pool)
	billingAdapter := NewGRPCServer(svc)

	grpcServer := grpc.NewServer()
	billing_pb.RegisterBillingServiceServer(grpcServer, billingAdapter)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen on port 50051: %v\n", err)
	}

	log.Println("Server is listening on :50051")
	if err = grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to server gRPC: %v\n", err)
	}
}
