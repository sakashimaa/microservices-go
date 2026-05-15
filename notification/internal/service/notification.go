package service

import (
	"context"
	"fmt"

	"gopkg.in/gomail.v2"
)

type NotificationService interface {
	SendDepositSuccessEmail(ctx context.Context, email string, amount int64) error
	SendWelcomeEmail(ctx context.Context, email string) error
}

type EmailService struct {
	dialer *gomail.Dialer
}

func NewNotificationService(dialer *gomail.Dialer) NotificationService {
	return &EmailService{
		dialer: dialer,
	}
}

func (s *EmailService) SendWelcomeEmail(ctx context.Context, email string) error {
	message := gomail.NewMessage()

	message.SetHeader("From", s.dialer.Username)
	message.SetHeader("To", email)
	message.SetHeader("Subject", "Welcome to our service!")

	message.SetBody("text/plain", fmt.Sprintf("Dear %s, thank you for registering in our billing service. Best regards", email))
	if err := s.dialer.DialAndSend(message); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (s *EmailService) SendDepositSuccessEmail(ctx context.Context, email string, amount int64) error {
	message := gomail.NewMessage()

	message.SetHeader("From", s.dialer.Username)
	message.SetHeader("To", email)
	message.SetHeader("Subject", "Your deposit was succeeded")

	message.SetBody("text/plain", fmt.Sprintf("Your deposit %d was succeeded at our platform.", amount))

	if err := s.dialer.DialAndSend(message); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
