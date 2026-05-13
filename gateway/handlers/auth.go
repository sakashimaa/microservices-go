package handlers

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sakashimaa/billing-microservice/contracts/gen/auth_pb"
	"github.com/sakashimaa/billing-microservice/gateway/middleware"
	"github.com/sakashimaa/billing-microservice/pkg/utils/api"
	"github.com/sakashimaa/billing-microservice/pkg/utils/grpc"
	"github.com/sony/gobreaker"
)

type GetMeHTTPResponse struct {
	Id        string   `json:"id"`
	Email     string   `json:"email"`
	CreatedAt string   `json:"created_at"`
	Roles     []string `json:"roles"`
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
	cb        *gobreaker.CircuitBreaker
}

func NewAuthHandler(client auth_pb.AuthServiceClient, publicKey *rsa.PublicKey, cb *gobreaker.CircuitBreaker) *AuthHandler {
	return &AuthHandler{
		client:    client,
		publicKey: publicKey,
		cb:        cb,
	}
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.SendJSON(w, http.StatusBadRequest, "error", "invalid JSON body")
		return
	}

	result, err := h.cb.Execute(func() (interface{}, error) {
		return h.client.Refresh(r.Context(), &auth_pb.RefreshRequest{
			Refresh: req.RefreshToken,
		})
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			api.SendJSON(w, http.StatusServiceUnavailable, "error", "service unavailable. Try again later")
			return
		}

		grpc.HandleGRPCError(w, err)
		return
	}

	resp := result.(*auth_pb.RefreshResponse)
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

	result, err := h.cb.Execute(func() (interface{}, error) {
		return h.client.Login(r.Context(), &auth_pb.LoginRequest{
			Email:    req.Email,
			Password: req.Password,
		})
	})
	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			api.SendJSON(w, http.StatusServiceUnavailable, "error", "service unavailable. Try again later")
			return
		}

		grpc.HandleGRPCError(w, err)
		return
	}

	resp := result.(*auth_pb.LoginResponse)
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

	result, err := h.cb.Execute(func() (interface{}, error) {
		return h.client.Register(r.Context(), &auth_pb.RegisterRequest{
			Email:    req.Email,
			Password: req.Password,
		})
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			api.SendJSON(w, http.StatusServiceUnavailable, "error", "service unavailable. Try again later")
			return
		}

		grpc.HandleGRPCError(w, err)
		return
	}

	resp := result.(*auth_pb.RegisterResponse)
	api.SendJSON(w, http.StatusCreated, "success", RegisterHTTPResponse{
		ID:           resp.Id,
		Email:        resp.Email,
		CreatedAt:    resp.CreatedAt,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	})
}

func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	_, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok {
		api.SendJSON(w, http.StatusUnauthorized, "error", "invalid user id")
		return
	}

	result, err := h.cb.Execute(func() (interface{}, error) {
		return h.client.GetMe(r.Context(), &auth_pb.GetMeRequest{})
	})
	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			api.SendJSON(w, http.StatusServiceUnavailable, "error", "service unavailable. Try again later")
			return
		}

		grpc.HandleGRPCError(w, err)
		return
	}

	res := result.(*auth_pb.GetMeResponse)
	api.SendJSON(w, http.StatusOK, "success", GetMeHTTPResponse{
		Id:        res.Id,
		Email:     res.Email,
		CreatedAt: res.CreatedAt,
		Roles:     res.Roles,
	})
}
