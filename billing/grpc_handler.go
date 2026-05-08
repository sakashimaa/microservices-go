package main

import (
	"context"
	"errors"

	"github.com/sakashimaa/billing-microservice/billing/services"
	"github.com/sakashimaa/billing-microservice/contracts/gen/billing_pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GrpcServer struct {
	billing_pb.UnimplementedBillingServiceServer

	svc services.BillingService
}

func NewGRPCServer(svc services.BillingService) *GrpcServer {
	return &GrpcServer{svc: svc}
}

func (s *GrpcServer) Deposit(ctx context.Context, req *billing_pb.DepositRequest) (*billing_pb.DepositResponse, error) {
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be greater than 0")
	}

	err := s.svc.Deposit(ctx, req.UserId, req.Amount)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	return &billing_pb.DepositResponse{Success: true}, nil
}

func (s *GrpcServer) Withdraw(ctx context.Context, req *billing_pb.WithdrawRequest) (*billing_pb.WithdrawResponse, error) {
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be greater than 0")
	}

	err := s.svc.Withdraw(ctx, req.UserId, req.Amount)
	if err != nil {
		if errors.Is(err, services.ErrInsufficientFunds) {
			return nil, status.Error(codes.FailedPrecondition, "insufficient funds")
		}
		if errors.Is(err, services.ErrAccountNotFound) {
			return nil, status.Error(codes.NotFound, "account not found")
		}
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	return &billing_pb.WithdrawResponse{
		Success: true,
	}, nil
}
