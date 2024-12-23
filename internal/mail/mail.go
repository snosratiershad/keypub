package mail

import (
	"fmt"
	"os"
	"strings"

	"github.com/resend/resend-go/v2"
)

// MailSender handles email operations for KeyPub.sh
type MailSender struct {
	client    *resend.Client
	fromEmail string
	fromName  string
}

// NewMailSender creates a new MailSender instance
// keyPath is the path to the file containing the Resend API key
func NewMailSender(keyPath string, fromEmail string, fromName string) (*MailSender, error) {
	content, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("cannot load resend api key: %w", err)
	}

	apiKey := strings.TrimSpace(string(content))
	apiKey = strings.ReplaceAll(apiKey, "\n", " ")
	apiKey = strings.ReplaceAll(apiKey, "\r", "")

	return &MailSender{
		client:    resend.NewClient(apiKey),
		fromEmail: fromEmail,
		fromName:  fromName,
	}, nil
}

// SendConfirmation sends a confirmation email with the provided confirmation number
func (m *MailSender) SendConfirmation(to string, confirmationNumber string, keyFingerprint string) error {
	fromField := fmt.Sprintf("%s <%s>", m.fromName, m.fromEmail)

	htmlContent := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
			<h2>Welcome to KeyPub.sh!</h2>
			<p>Thank you for registering. You are confirming a key with fingerprint:</p>
			<div style="background-color: #f5f5f5; padding: 15px; border-radius: 5px; margin: 20px 0;">
				<p style="font-family: monospace; font-size: 16px; margin: 0;">%s...</p>
			</div>
			<p>To complete your registration, please use the confirmation number below:</p>
			<div style="background-color: #f5f5f5; padding: 15px; border-radius: 5px; margin: 20px 0;">
				<p style="font-size: 18px; margin: 0;">Your confirmation number: <strong>%s</strong></p>
			</div>
			<p>Run the following command:</p>
			<pre style="background-color: #f5f5f5; padding: 15px; border-radius: 5px; overflow-x: auto;">ssh keypub.sh confirm %s</pre>
			<p style="color: #666; margin-top: 20px; font-size: 14px;">
				If you didn't request this registration, please ignore this email.
			</p>
		</div>
	`, keyFingerprint, confirmationNumber, confirmationNumber)

	params := &resend.SendEmailRequest{
		From:    fromField,
		To:      []string{to},
		Subject: fmt.Sprintf("Complete KeyPub.sh Registration for Key %s...", keyFingerprint),
		Html:    htmlContent,
	}

	_, err := m.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("failed to send confirmation email: %w", err)
	}

	return nil
}
