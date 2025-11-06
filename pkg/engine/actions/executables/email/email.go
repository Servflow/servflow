package email

import (
	"context"
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/Servflow/servflow/pkg/engine/actions"
)

type Email struct {
	cfg *Config
}

func (e *Email) Type() string {
	return "email"
}

type Config struct {
	SenderEmail    string       `json:"senderEmail"`
	RecipientEmail string       `json:"recipientEmail"`
	Name           string       `json:"name"`
	Subject        string       `json:"subject,omitempty"`
	ServerConfig   ServerConfig `json:"auth"`
	Content        []byte       `json:"content"`
}

type ServerConfig struct {
	ServerHost string `json:"serverHostname"`
	ServerPort string `json:"serverPort"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

func New(cfg Config) *Email {
	return &Email{
		cfg: &cfg,
	}
}

func (e *Email) Config() string {
	configBytes, err := json.Marshal(e.cfg)
	if err != nil {
		return ""
	}

	return string(configBytes)
}

func (e *Email) Execute(ctx context.Context, filledInConfig string) (interface{}, error) {
	var cfg Config

	if err := json.Unmarshal([]byte(filledInConfig), &cfg); err != nil {
		return nil, err
	}

	message := strings.Builder{}

	message.WriteString(fmt.Sprintf("From:%s\r\nTo:%s\r\n", cfg.SenderEmail, cfg.RecipientEmail))

	if cfg.Subject != "" {
		message.WriteString(fmt.Sprintf("Subject: %s\r\n", cfg.Subject))
	}

	message.WriteString("\r\n")

	message.WriteString(string(cfg.Content))

	auth := smtp.PlainAuth("", cfg.ServerConfig.Username, cfg.ServerConfig.Password, cfg.ServerConfig.ServerHost)

	err := smtp.SendMail(
		fmt.Sprintf("%s:%s", cfg.ServerConfig.ServerHost, cfg.ServerConfig.ServerPort),
		auth,
		cfg.SenderEmail,
		[]string{cfg.RecipientEmail},
		[]byte(message.String()),
	)

	if err != nil {
		return nil, err
	}

	return nil, nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"senderEmail": {
			Type:        "string",
			Label:       "Sender Email",
			Placeholder: "sender@example.com",
			Required:    true,
		},
		"recipientEmail": {
			Type:        "string",
			Label:       "Recipient Email",
			Placeholder: "recipient@example.com",
			Required:    true,
		},
		"name": {
			Type:        "string",
			Label:       "Sender Name",
			Placeholder: "John Doe",
			Required:    true,
		},
		"subject": {
			Type:        "string",
			Label:       "Subject",
			Placeholder: "Email subject",
			Required:    false,
		},
		"auth": {
			Type:        "object",
			Label:       "Server Configuration",
			Placeholder: "SMTP server authentication details",
			Required:    true,
		},
		"content": {
			Type:        "string",
			Label:       "Content",
			Placeholder: "Email content",
			Required:    true,
		},
	}

	if err := actions.RegisterAction("email", func(config json.RawMessage) (actions.ActionExecutable, error) {
		var cfg Config
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, fmt.Errorf("error creating email action: %v", err)
		}
		return New(cfg), nil
	}, fields); err != nil {
		panic(err)
	}
}
