package main

import (
	"log"
	"net/http"

	"github.com/sakashimaa/billing-microservice/contracts/gen/billing_pb"
	"github.com/sakashimaa/billing-microservice/gateway/handlers"
	"github.com/sakashimaa/billing-microservice/pkg/utils/env"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Gateway struct {
	billingClient billing_pb.BillingServiceClient
}

func main() {
	addr := env.ParseEnvWithFallback("BILLING_ADDR", "localhost:50051")

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to billing using gRPC: %v\n", err)
	}
	defer conn.Close()

	client := billing_pb.NewBillingServiceClient(conn)
	billingHandler := handlers.NewBillingHandler(client)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /deposit", http.HandlerFunc(billingHandler.DepositHandler))
	mux.HandleFunc("POST /withdraw", http.HandlerFunc(billingHandler.WithdrawalHandler))

	log.Println("Gateway started on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("failed to listen on :8080: %v\n", err)
	}
}
