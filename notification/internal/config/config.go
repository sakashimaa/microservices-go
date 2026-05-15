package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

type NotificationConfig struct {
	DbUrl     string `env:"DATABASE_URL"`
	BrokerUrl string `env:"BROKER_URL"`
	SmtpPort  int    `env:"SMTP_PORT"`
	SmtpEmail string `env:"SMTP_EMAIL"`
	SmtpHost  string `env:"SMTP_HOST"`
	SmtpPass  string `env:"SMTP_PASSWORD"`
}

func NewConfig(path string) (*NotificationConfig, error) {
	var res NotificationConfig

	if err := cleanenv.ReadConfig(path, &res); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	return &res, nil
}
