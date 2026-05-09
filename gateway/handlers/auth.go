package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/sakashimaa/billing-microservice/contracts/gen/auth_pb"
	"github.com/sakashimaa/billing-microservice/pkg/utils/api"
	"github.com/sakashimaa/billing-microservice/pkg/utils/grpc"
)

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterHTTPResponse struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	CreatedAt    string `json:"created_at"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthHandler struct {
	client auth_pb.AuthServiceClient
}

func NewAuthHandler(client auth_pb.AuthServiceClient) *AuthHandler {
	return &AuthHandler{
		client: client,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.SendJSON(w, 404, "error", "invalid JSON body")
		return
	}

	resp, err := h.client.Register(r.Context(), &auth_pb.RegisterRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		grpc.HandleGRPCError(w, err)
		return
	}

	api.SendJSON(w, 201, "success", RegisterHTTPResponse{
		ID:           resp.Id,
		Email:        resp.Email,
		CreatedAt:    resp.CreatedAt,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	})
}
