package auth

import (
	"fmt"
	"net/smtp"
	"strings"
)

// sendMail sends a simple plaintext email using SMTP (Gmail)
// emailKey is the app password
func SendVerificationEmail(smtpUser, smtpPass, to, verifyURL string) error {
	// Using Gmail SMTP
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"
	from := smtpUser
	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)

	subject := "Verify your email"
	body := fmt.Sprintf("Please verify your account by clicking the link: %s", verifyURL)
	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"",
		body,
	}, "\r\n")

	return smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{to}, []byte(msg))
}
