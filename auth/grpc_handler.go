package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sakashimaa/billing-microservice/auth/domain"
	"github.com/sakashimaa/billing-microservice/auth/services"
	"github.com/sakashimaa/billing-microservice/contracts/gen/auth_pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GrpcServer struct {
	auth_pb.UnimplementedAuthServiceServer
	service services.AuthService
}

func NewGRPCHandler(service services.AuthService) *GrpcServer {
	return &GrpcServer{
		service: service,
	}
}

func (s *GrpcServer) Register(ctx context.Context, req *auth_pb.RegisterRequest) (*auth_pb.RegisterResponse, error) {
	res, err := s.service.Register(ctx, domain.RegisterRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrUserAlreadyExists):
			return nil, status.Errorf(codes.AlreadyExists, err.Error())
		case errors.Is(err, services.ErrInvalidInput):
			return nil, status.Errorf(codes.InvalidArgument, err.Error())
		default:
			fmt.Printf("internal server error: %v", err)
			return nil, status.Error(codes.Internal, "internal server error")
		}
	}

	return &auth_pb.RegisterResponse{
		Id:           res.Id,
		Email:        res.Email,
		CreatedAt:    res.CreatedAt.Format(time.RFC3339),
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
	}, nil
}
