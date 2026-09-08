package services

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"gobackend/config"
)

type EmailService struct {
	cfg *config.Config
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{cfg: cfg}
}

// SendWelcomeEmail sends a welcome/confirmation email to the newly registered user via Gmail SMTP
func (s *EmailService) SendWelcomeEmail(toEmail, fullName string) error {
	if s.cfg.GmailUser == "" || s.cfg.GmailAppPassword == "" {
		log.Printf("Notice: Gmail SMTP credentials (GMAIL_USER / GMAIL_APP_PASSWORD) not configured. Skipping welcome email to %s\n", toEmail)
		return nil
	}

	subject := "Welcome to Assessment Platform!"
	body := fmt.Sprintf(`Hello %s,

Thank you for signing up on the Assessment Platform. Your account has been successfully created.

Best regards,
Assessment Team`, fullName)

	return s.sendMail(toEmail, subject, body)
}

// sendMail connects to Gmail SMTP server (smtp.gmail.com:587) and sends email message
func (s *EmailService) sendMail(toEmail, subject, body string) error {
	auth := smtp.PlainAuth("", s.cfg.GmailUser, s.cfg.GmailAppPassword, s.cfg.SMTPHost)

	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)

	header := make(map[string]string)
	header["From"] = fmt.Sprintf("Assessment Platform <%s>", s.cfg.GmailUser)
	header["To"] = toEmail
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=\"utf-8\""

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + strings.ReplaceAll(body, "\n", "\r\n")

	err := smtp.SendMail(addr, auth, s.cfg.GmailUser, []string{toEmail}, []byte(message))
	if err != nil {
		log.Printf("Error sending email via Gmail SMTP to %s: %v\n", toEmail, err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Successfully sent Gmail email to %s\n", toEmail)
	return nil
}
