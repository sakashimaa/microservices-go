package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/sakashimaa/billing-microservice/contracts/gen/billing_pb"
	"github.com/sakashimaa/billing-microservice/pkg/utils/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BillingRequest struct {
	UserID int64 `json:"user_id"`
	Amount int64 `json:"amount"`
}

type BillingHandler struct {
	client billing_pb.BillingServiceClient
}

func NewBillingHandler(client billing_pb.BillingServiceClient) *BillingHandler {
	return &BillingHandler{
		client: client,
	}
}

func (h *BillingHandler) WithdrawalHandler(w http.ResponseWriter, r *http.Request) {
	var req billing_pb.WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.SendJSON(w, http.StatusBadRequest, "error", "invalid json body")
		return
	}

	resp, err := h.client.Withdraw(r.Context(), &billing_pb.WithdrawRequest{
		Amount: req.Amount,
		UserId: req.UserId,
	})
	if err != nil {
		h.handleGRPCError(w, err)
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
		Amount: req.Amount,
		UserId: req.UserID,
	})

	if err != nil {
		h.handleGRPCError(w, err)
		return
	}

	api.SendJSON(w, http.StatusOK, "success", map[string]bool{"success": resp.Success})
}

func (h *BillingHandler) handleGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		api.SendJSON(w, http.StatusInternalServerError, "error", "internal server error")
		return
	}

	switch st.Code() {
	case codes.InvalidArgument:
		api.SendJSON(w, http.StatusBadRequest, "error", st.Message())
	case codes.NotFound:
		api.SendJSON(w, http.StatusNotFound, "error", st.Message())
	case codes.FailedPrecondition:
		api.SendJSON(w, http.StatusBadRequest, "error", st.Message())
	default:
		log.Printf("grpc error: %v", st.Message())
		api.SendJSON(w, http.StatusInternalServerError, "error", st.Message())
	}
}
