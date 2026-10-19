package app

import (
	"context"
	"errors"
	"testing"

	"southpark-messenger/services/api/internal/domain"
)

type publisherStub struct {
	message domain.Message
	err     error
	calls   int
}

func (p *publisherStub) Publish(_ context.Context, message domain.Message) error {
	p.calls++
	p.message = message
	return p.err
}

func TestMessageServiceSendPublishesValidMessage(t *testing.T) {
	publisher := &publisherStub{}
	service := NewMessageService(publisher)

	if err := service.Send(context.Background(), " Stan ", " Dude! "); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if publisher.calls != 1 {
		t.Fatalf("Publish() calls = %d, want 1", publisher.calls)
	}
	want := domain.Message{Author: "Stan", Body: "Dude!"}
	if publisher.message != want {
		t.Fatalf("published message = %#v, want %#v", publisher.message, want)
	}
}

func TestMessageServiceSendDoesNotPublishInvalidMessage(t *testing.T) {
	publisher := &publisherStub{}
	service := NewMessageService(publisher)

	err := service.Send(context.Background(), "", "Dude!")
	if !errors.Is(err, domain.ErrAuthorRequired) {
		t.Fatalf("Send() error = %v, want %v", err, domain.ErrAuthorRequired)
	}
	if publisher.calls != 0 {
		t.Fatalf("Publish() calls = %d, want 0", publisher.calls)
	}
}

func TestMessageServiceSendWrapsPublisherError(t *testing.T) {
	publishErr := errors.New("broker is unavailable")
	publisher := &publisherStub{err: publishErr}
	service := NewMessageService(publisher)

	err := service.Send(context.Background(), "Kyle", "You bastards!")
	if !errors.Is(err, publishErr) {
		t.Fatalf("Send() error = %v, want wrapped %v", err, publishErr)
	}
}
