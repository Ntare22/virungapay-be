package mail

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"virungapay-be/internal/config"
)

const mailjetSendURL = "https://api.mailjet.com/v3.1/send"

// Mailjet sends through the Mailjet Send API v3.1.
type Mailjet struct {
	apiKey    string
	secretKey string
	from      string
	client    *http.Client
}

// NewMailjet builds a sender from process configuration.
func NewMailjet(cfg config.MailConfig) *Mailjet {
	return &Mailjet{
		apiKey:    cfg.APIKey,
		secretKey: cfg.SecretKey,
		from:      cfg.From,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

type sendBody struct {
	Messages []sendMessage `json:"Messages"`
}

type sendMessage struct {
	From        address          `json:"From"`
	To          []address        `json:"To"`
	Subject     string           `json:"Subject"`
	TextPart    string           `json:"TextPart"`
	HTMLPart    string           `json:"HTMLPart,omitempty"`
	Attachments []sendAttachment `json:"Attachments,omitempty"`
}

type sendAttachment struct {
	ContentType   string `json:"ContentType"`
	Filename      string `json:"Filename"`
	Base64Content string `json:"Base64Content"`
}

type address struct {
	Email string `json:"Email"`
	Name  string `json:"Name,omitempty"`
}

// Send posts one message. The raw token is only present in the message body the caller built.
func (m *Mailjet) Send(ctx context.Context, message Message) error {
	if strings.TrimSpace(m.apiKey) == "" || strings.TrimSpace(m.secretKey) == "" {
		fmt.Printf("email failed to=%s subject=%q error=%q\n", message.ToEmail, message.Subject, "mailjet credentials are not configured")
		return fmt.Errorf("mailjet credentials are not configured")
	}
	files := make([]sendAttachment, 0, len(message.Attachments))
	for _, file := range message.Attachments {
		files = append(files, sendAttachment{
			ContentType: file.ContentType, Filename: file.Filename, Base64Content: base64.StdEncoding.EncodeToString(file.Content),
		})
	}
	payload, err := json.Marshal(sendBody{Messages: []sendMessage{{
		From:        address{Email: m.from, Name: "VirungaPay"},
		To:          []address{{Email: message.ToEmail, Name: message.ToName}},
		Subject:     message.Subject,
		TextPart:    message.Text,
		HTMLPart:    message.HTML,
		Attachments: files,
	}}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mailjetSendURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.SetBasicAuth(m.apiKey, m.secretKey)
	req.Header.Set("Content-Type", "application/json")
	fmt.Printf("email sending to=%s subject=%q from=%s\n", message.ToEmail, message.Subject, m.from)
	res, err := m.client.Do(req)
	if err != nil {
		fmt.Printf("email failed to=%s subject=%q error=%q\n", message.ToEmail, message.Subject, err.Error())
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	outcome := readOutcome(body)
	fmt.Printf("email result to=%s subject=%q http=%d status=%s error=%q\n", message.ToEmail, message.Subject, res.StatusCode, outcome.status, outcome.errText)
	log := zerolog.Ctx(ctx).Info()
	if res.StatusCode < 200 || res.StatusCode >= 300 || outcome.failed {
		log = zerolog.Ctx(ctx).Error()
	}
	log.Str("to", message.ToEmail).
		Str("subject", message.Subject).
		Int("mailjet_status", res.StatusCode).
		Str("mailjet_message_status", outcome.status).
		Str("mailjet_error", outcome.errText).
		Msg("mailjet send")
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("mailjet status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	if outcome.failed {
		return fmt.Errorf("mailjet rejected the message: %s", outcome.errText)
	}
	return nil
}

type mailOutcome struct {
	status  string
	errText string
	failed  bool
}

func readOutcome(body []byte) mailOutcome {
	var parsed struct {
		ErrorMessage string `json:"ErrorMessage"`
		Messages     []struct {
			Status string `json:"Status"`
			Errors []struct {
				ErrorMessage string `json:"ErrorMessage"`
			} `json:"Errors"`
		} `json:"Messages"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return mailOutcome{}
	}
	if parsed.ErrorMessage != "" {
		return mailOutcome{status: "error", errText: parsed.ErrorMessage, failed: true}
	}
	if len(parsed.Messages) == 0 {
		return mailOutcome{}
	}
	message := parsed.Messages[0]
	parts := make([]string, 0, len(message.Errors))
	for _, item := range message.Errors {
		if item.ErrorMessage != "" {
			parts = append(parts, item.ErrorMessage)
		}
	}
	failed := strings.EqualFold(message.Status, "error")
	return mailOutcome{status: message.Status, errText: strings.Join(parts, "; "), failed: failed}
}
