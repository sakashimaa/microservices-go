package domain

import "time"

type DepositRequest struct {
	UserId         string
	Amount         int64
	IdempotencyKey string
}

type DepositResponse struct{}

type WithdrawRequest struct {
	UserId         string
	Amount         int64
	IdempotencyKey string
}

type WithdrawResponse struct{}

type TopUpAccountParams struct {
	UserId string
	Amount int64
}

type InsertTransactionParams struct {
	AccountId      string
	Amount         int64
	Type           string
	IdempotencyKey string
}

type FindByIdempotencyKeyParams struct {
	IdempotencyKey string
}

type Transaction struct {
	Id             string
	AccountId      string
	Amount         int64
	Type           string
	IdempotencyKey string
	CreatedAt      *time.Time
}

type GetAccountByUserIdParams struct {
	UserId string
}

type Account struct {
	Id        string
	UserId    string
	Balance   int64
	CreatedAt *time.Time
}

type WithdrawAccountParams struct {
	Amount int64
	Id     string
}
