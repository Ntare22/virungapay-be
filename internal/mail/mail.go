// Package mail sends verification, password-reset, and invite messages.
// Mailjet is the only provider. Tests inject a Sender that records messages.
package mail

import "context"

// Message is one outbound email.
type Message struct {
	ToEmail     string
	ToName      string
	Subject     string
	Text        string
	HTML        string
	Attachments []Attachment
}

// Attachment is a file sent with the message.
type Attachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

// Sender delivers one message.
type Sender interface {
	Send(ctx context.Context, message Message) error
}
