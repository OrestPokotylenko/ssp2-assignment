package ports

import "context"

// MessageUseCase is the inbound port used by delivery adapters such as HTTP.
type MessageUseCase interface {
	Send(ctx context.Context, author, body string) error
}
