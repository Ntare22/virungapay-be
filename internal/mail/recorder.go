package mail

import (
	"context"
	"regexp"
	"sync"
)

// Recorder stores messages so tests can read emailed links without calling Mailjet.
type Recorder struct {
	mu       sync.Mutex
	Messages []Message
}

// Send appends the message.
func (r *Recorder) Send(_ context.Context, message Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Messages = append(r.Messages, message)
	return nil
}

var tokenPattern = regexp.MustCompile(`token=([A-Za-z0-9]+)`)
var codePattern = regexp.MustCompile(`code is (\d{6})`)

// TokenFor returns the token from the latest message sent to email.
func (r *Recorder) TokenFor(email string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.Messages) - 1; i >= 0; i-- {
		if r.Messages[i].ToEmail != email {
			continue
		}
		match := tokenPattern.FindStringSubmatch(r.Messages[i].Text)
		if len(match) == 2 {
			return match[1]
		}
	}
	return ""
}

// CodeFor returns the six-digit code from the latest message sent to email.
func (r *Recorder) CodeFor(email string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.Messages) - 1; i >= 0; i-- {
		if r.Messages[i].ToEmail != email {
			continue
		}
		match := codePattern.FindStringSubmatch(r.Messages[i].Text)
		if len(match) == 2 {
			return match[1]
		}
	}
	return ""
}
