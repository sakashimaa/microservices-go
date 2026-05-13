package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/sakashimaa/billing-microservice/contracts/gen/auth_pb"
	"github.com/sakashimaa/billing-microservice/contracts/gen/billing_pb"
	"github.com/sakashimaa/billing-microservice/gateway/handlers"
	"github.com/sakashimaa/billing-microservice/gateway/middleware"
	"github.com/sakashimaa/billing-microservice/pkg/utils/env"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	_ = godotenv.Load()

	redisUrl := env.ParseEnvWithFallback("REDIS_URL", "localhost:6379")

	rdb := redis.NewClient(&redis.Options{
		Addr: redisUrl,
	})
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Fatalf("failed to close redis: %v", err)
		}
	}()

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("failed to ping redis: %v", err)
	}

	billingAddr := env.ParseEnvWithFallback("BILLING_ADDR", "localhost:50051")
	authAddr := env.ParseEnvWithFallback("AUTH_ADDR", "localhost:50052")

	billingConn, err := grpc.NewClient(billingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to billing using gRPC: %v\n", err)
	}
	defer func(billingConn *grpc.ClientConn) {
		if err = billingConn.Close(); err != nil {
			fmt.Printf("failed to close connection to billing gRPC: %v\n", err)
		}
	}(billingConn)

	authConn, err := grpc.NewClient(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to auth using gRPC: %v\n", err)
	}
	defer func(authConn *grpc.ClientConn) {
		if err = authConn.Close(); err != nil {
			fmt.Printf("failed to close connection to auth gRPC: %v\n", err)
		}
	}(authConn)

	publicKeyBytes, err := os.ReadFile("public.pem")
	if err != nil {
		log.Fatalf("failed to read public key file: %v", err)
	}
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(publicKeyBytes)
	if err != nil {
		log.Fatalf("failed to parse public key from pem: %v", err)
	}

	billingClient := billing_pb.NewBillingServiceClient(billingConn)
	billingHandler := handlers.NewBillingHandler(billingClient, publicKey)

	authClient := auth_pb.NewAuthServiceClient(authConn)

	authHandler := handlers.NewAuthHandler(authClient, publicKey)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /register", authHandler.Register)
	mux.HandleFunc("POST /login", authHandler.Login)
	mux.HandleFunc("POST /refresh", authHandler.Refresh)

	mux.Handle("GET /me", middleware.AuthMiddleware(publicKey)(http.HandlerFunc(authHandler.GetMe)))
	mux.Handle("POST /deposit", middleware.AuthMiddleware(publicKey)(http.HandlerFunc(billingHandler.DepositHandler)))
	mux.Handle("POST /withdraw", middleware.AuthMiddleware(publicKey)(http.HandlerFunc(billingHandler.WithdrawalHandler)))

	globalHandler := middleware.RateLimitMiddleware(rdb, 50, time.Minute)(mux)

	log.Println("Gateway started on :8080")
	if err := http.ListenAndServe(":8080", globalHandler); err != nil {
		log.Fatalf("failed to listen on :8080: %v\n", err)
	}
}
