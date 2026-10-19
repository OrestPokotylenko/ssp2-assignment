package ports

import (
	"context"

	"southpark-messenger/services/api/internal/domain"
)

// MessagePublisher is the outbound port used by the application layer.
// Infrastructure adapters such as RabbitMQ implement this interface.
type MessagePublisher interface {
	Publish(ctx context.Context, message domain.Message) error
}
