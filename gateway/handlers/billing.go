package handlers

import (
	"crypto/rsa"
	"encoding/json"
	"net/http"

	"github.com/sakashimaa/billing-microservice/contracts/gen/billing_pb"
	"github.com/sakashimaa/billing-microservice/pkg/utils/api"
	"github.com/sakashimaa/billing-microservice/pkg/utils/grpc"
)

type BillingRequest struct {
	UserID         string `json:"user_id"`
	Amount         int64  `json:"amount"`
	IdempotencyKey string `json:"idempotency_key"`
}

type BillingHandler struct {
	client    billing_pb.BillingServiceClient
	publicKey *rsa.PublicKey
}

func NewBillingHandler(client billing_pb.BillingServiceClient, publicKey *rsa.PublicKey) *BillingHandler {
	return &BillingHandler{
		client:    client,
		publicKey: publicKey,
	}
}

func (h *BillingHandler) WithdrawalHandler(w http.ResponseWriter, r *http.Request) {
	var req billing_pb.WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.SendJSON(w, http.StatusBadRequest, "error", "invalid json body")
		return
	}

	resp, err := h.client.Withdraw(r.Context(), &billing_pb.WithdrawRequest{
		Amount:         req.Amount,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		grpc.HandleGRPCError(w, err)
		return
	}

	api.SendJSON(w, http.StatusOK, "success", map[string]bool{"success": resp.Success})
}

func (h *BillingHandler) DepositHandler(w http.ResponseWriter, r *http.Request) {
	var req BillingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.SendJSON(w, http.StatusBadRequest, "error", "invalid json body")
		return
	}

	resp, err := h.client.Deposit(r.Context(), &billing_pb.DepositRequest{
		Amount:         req.Amount,
		IdempotencyKey: req.IdempotencyKey,
	})

	if err != nil {
		grpc.HandleGRPCError(w, err)
		return
	}

	api.SendJSON(w, http.StatusOK, "success", map[string]bool{"success": resp.Success})
}
