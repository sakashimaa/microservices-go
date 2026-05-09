package domain

import "time"

type User struct {
	Id        string
	Email     string
	Password  string
	CreatedAt time.Time
}

type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

type RegisterRequest struct {
	Email    string
	Password string
}

type CreateUserParams struct {
	Email        string
	PasswordHash string
}

type RegisterResponse struct {
	Id           string
	Email        string
	CreatedAt    time.Time
	AccessToken  string
	RefreshToken string
}
