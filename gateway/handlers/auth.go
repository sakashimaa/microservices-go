package handlers

import (
	"crypto/rsa"
	"encoding/json"
	"net/http"

	"github.com/sakashimaa/billing-microservice/contracts/gen/auth_pb"
	"github.com/sakashimaa/billing-microservice/gateway/middleware"
	"github.com/sakashimaa/billing-microservice/pkg/utils/api"
	"github.com/sakashimaa/billing-microservice/pkg/utils/grpc"
)

type GetMeHTTPResponse struct {
	Id        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshHTTPResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginHTTPResponse struct {
	Id           string `json:"id"`
	Email        string `json:"email"`
	CreatedAt    string `json:"created_at"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

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
	client    auth_pb.AuthServiceClient
	publicKey *rsa.PublicKey
}

func NewAuthHandler(client auth_pb.AuthServiceClient, publicKey *rsa.PublicKey) *AuthHandler {
	return &AuthHandler{
		client:    client,
		publicKey: publicKey,
	}
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.SendJSON(w, http.StatusBadRequest, "error", "invalid JSON body")
		return
	}

	resp, err := h.client.Refresh(r.Context(), &auth_pb.RefreshRequest{
		Refresh: req.RefreshToken,
	})
	if err != nil {
		grpc.HandleGRPCError(w, err)
		return
	}

	api.SendJSON(w, http.StatusOK, "success", RefreshHTTPResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.SendJSON(w, http.StatusBadRequest, "error", "invalid JSON body")
		return
	}

	resp, err := h.client.Login(r.Context(), &auth_pb.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		grpc.HandleGRPCError(w, err)
		return
	}

	api.SendJSON(w, http.StatusOK, "success", LoginHTTPResponse{
		Id:           resp.Id,
		Email:        resp.Email,
		CreatedAt:    resp.CreatedAt,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.SendJSON(w, http.StatusBadRequest, "error", "invalid JSON body")
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

	api.SendJSON(w, http.StatusCreated, "success", RegisterHTTPResponse{
		ID:           resp.Id,
		Email:        resp.Email,
		CreatedAt:    resp.CreatedAt,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	})
}

func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	enrichedReq, err := middleware.ValidateToken(r, h.publicKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	res, err := h.client.GetMe(enrichedReq.Context(), &auth_pb.GetMeRequest{})
	if err != nil {
		grpc.HandleGRPCError(w, err)
		return
	}

	api.SendJSON(w, http.StatusOK, "success", GetMeHTTPResponse{
		Id:        res.Id,
		Email:     res.Email,
		CreatedAt: res.CreatedAt,
	})
}
