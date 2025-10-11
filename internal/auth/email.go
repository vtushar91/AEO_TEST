package auth

import (
	"fmt"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// SendVerificationEmailSendGrid sends a verification email using SendGrid
// emailFrom: verified sender email
// apiKey: SendGrid API key
// toEmail: recipient email
// verifyURL: verification link
func SendVerificationEmail(emailFrom, apiKey, toEmail, verifyURL string) error {
	from := mail.NewEmail("Your App", emailFrom)
	subject := "Verify your account"
	to := mail.NewEmail("", toEmail)
	plainText := fmt.Sprintf("Click here to verify your account: %s", verifyURL)
	htmlContent := fmt.Sprintf(`<p>Please verify your account by clicking the link below:</p>
	<a href="%s">Verify Your Account</a>
	<p>If the link doesn’t work, copy and paste this URL into your browser:</p>
	<p>%s</p>`, verifyURL, verifyURL)

	message := mail.NewSingleEmail(from, subject, to, plainText, htmlContent)
	client := sendgrid.NewSendClient(apiKey)

	resp, err := client.Send(message)
	if err != nil {
		return fmt.Errorf("SendGrid error: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("SendGrid API returned status %d: %s", resp.StatusCode, resp.Body)
	}

	return nil
}
