package domain

import (
	"errors"
	"strings"
)

var (
	ErrAuthorRequired = errors.New("author is required")
	ErrBodyRequired   = errors.New("body is required")
)

type Message struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}

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
