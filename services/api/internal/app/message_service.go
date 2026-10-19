package app

import (
	"context"
	"fmt"

	"southpark-messenger/services/api/internal/domain"
	"southpark-messenger/services/api/internal/ports"
)

type MessageService struct {
	publisher ports.MessagePublisher
}

func NewMessageService(publisher ports.MessagePublisher) *MessageService {
	return &MessageService{publisher: publisher}
}

func (s *MessageService) Send(ctx context.Context, author, body string) error {
	message, err := domain.NewMessage(author, body)
	if err != nil {
		return err
	}

	if err := s.publisher.Publish(ctx, message); err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	return nil
}

var _ ports.MessageUseCase = (*MessageService)(nil)
