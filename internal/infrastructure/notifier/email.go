package notifier

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/datdt/k8sselfhost/internal/domain/alert"
)

type EmailNotifier struct{}

func NewEmailNotifier() *EmailNotifier {
	return &EmailNotifier{}
}

func (n *EmailNotifier) Send(ctx context.Context, channel *alert.NotificationChannel, message string) error {
	host, _ := channel.Config["smtp_host"].(string)
	port, _ := channel.Config["smtp_port"].(string)
	user, _ := channel.Config["smtp_user"].(string)
	pass, _ := channel.Config["smtp_pass"].(string)
	to, _ := channel.Config["to_email"].(string)
	from, _ := channel.Config["from_email"].(string)

	if host == "" || port == "" || to == "" || from == "" {
		return fmt.Errorf("incomplete email configuration")
	}

	auth := smtp.PlainAuth("", user, pass, host)
	msg := []byte("To: " + to + "\r\n" +
		"From: " + from + "\r\n" +
		"Subject: Alert Notification\r\n" +
		"\r\n" + message + "\r\n")

	addr := fmt.Sprintf("%s:%s", host, port)
	err := smtp.SendMail(addr, auth, from, []string{to}, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
