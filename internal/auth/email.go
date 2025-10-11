package auth

import (
	"fmt"

	"gopkg.in/gomail.v2"
)

// SendVerificationEmail sends a plaintext verification email using Gmail SMTP.
// smtpUser: Gmail email address
// smtpPass: Gmail App Password
// to: recipient email
// verifyURL: link for email verification
func SendVerificationEmail(smtpUser, smtpPass, to, verifyURL string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", smtpUser)
	m.SetHeader("To", to)
	m.SetHeader("Subject", "Verify your account")
	m.SetBody("text/plain", fmt.Sprintf("Please verify your account by clicking this link: %s", verifyURL))

	d := gomail.NewDialer("smtp.gmail.com", 587, smtpUser, smtpPass)
	// TLS is automatically handled by gomail

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
