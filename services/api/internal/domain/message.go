package domain

import (
	"errors"
	"strings"
)

var (
	ErrAuthorRequired = errors.New("author is required")
	ErrBodyRequired   = errors.New("body is required")
)

// Message is the payload placed on the South Park message queue.
type Message struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}

// NewMessage creates a valid message. Keeping validation in the domain means
// every adapter is subject to the same rules.
func NewMessage(author, body string) (Message, error) {
	author = strings.TrimSpace(author)
	body = strings.TrimSpace(body)

	if author == "" {
		return Message{}, ErrAuthorRequired
	}

	if body == "" {
		return Message{}, ErrBodyRequired
	}

	return Message{Author: author, Body: body}, nil
}
