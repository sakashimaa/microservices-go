package domain

import "time"

type User struct {
	Id        string
	Email     string
	Password  string
	CreatedAt *time.Time
}

type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  *time.Time
	RefreshExpiresAt *time.Time
}

type RegisterRequest struct {
	Email    string
	Password string
}

type LoginRequest struct {
	Email    string
	Password string
}

type CreateUserParams struct {
	Email        string
	PasswordHash string
}

type GetUserByEmailParams struct {
	Email string
}

type RegisterResponse struct {
	Id           string
	Email        string
	CreatedAt    *time.Time
	AccessToken  string
	RefreshToken string
}

type LoginResponse struct {
	Id           string
	Email        string
	CreatedAt    *time.Time
	AccessToken  string
	RefreshToken string
}

type SaveTokenParams struct {
	UserId     string
	Token      string
	UserAgent  string
	IpAddress  string
	ExpiresAt  *time.Time
	Revoked    bool
	ConsumedAt *time.Time
	RevokedBy  string
}

type Token struct {
	Id         string
	UserId     string
	Token      string
	UserAgent  string
	IpAddress  string
	ExpiresAt  *time.Time
	Revoked    bool
	ConsumedAt *time.Time
	RevokedBy  *string
	CreatedAt  *time.Time
}

type RefreshRequest struct {
	RefreshToken string
}

type RefreshResponse struct {
	AccessToken  string
	RefreshToken string
}

type GetMeRequest struct {
	UserId string
}

type GetMeResponse struct {
	Id        string
	Email     string
	CreatedAt *time.Time
}
