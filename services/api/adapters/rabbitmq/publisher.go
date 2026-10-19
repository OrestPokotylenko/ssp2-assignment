package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"

	"southpark-messenger/services/api/internal/domain"
	"southpark-messenger/services/api/internal/ports"
)

type Publisher struct {
	connection *amqp.Connection
	channel    *amqp.Channel
	queueName  string
	mu         sync.Mutex
	closeOnce  sync.Once
}

func NewPublisher(url, queueName string) (*Publisher, error) {
	if url == "" {
		return nil, errors.New("RabbitMQ URL is required")
	}

	if queueName == "" {
		return nil, errors.New("RabbitMQ queue name is required")
	}

	connection, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("open RabbitMQ channel: %w", err)
	}

	_, err = channel.QueueDeclare(
		queueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("declare RabbitMQ queue: %w", err)
	}

	if err := channel.Confirm(false); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("enable RabbitMQ publisher confirms: %w", err)
	}

	return &Publisher{
		connection: connection,
		channel:    channel,
		queueName:  queueName,
	}, nil
}

func (p *Publisher) Publish(ctx context.Context, message domain.Message) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}

	// Serialize access to the channel. It also keeps Close from racing with an
	// in-flight publish during graceful shutdown.
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connection.IsClosed() {
		return errors.New("RabbitMQ connection is closed")
	}

	confirmation, err := p.channel.PublishWithDeferredConfirmWithContext(
		ctx,
		"",          // default exchange
		p.queueName, // routing key
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish to queue %q: %w", p.queueName, err)
	}

	acknowledged, err := confirmation.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait for RabbitMQ confirmation: %w", err)
	}
	if !acknowledged {
		return errors.New("RabbitMQ rejected the message")
	}

	return nil
}

func (p *Publisher) Close() error {
	var closeErr error
	p.closeOnce.Do(func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		if err := p.channel.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) {
			closeErr = fmt.Errorf("close RabbitMQ channel: %w", err)
		}

		if err := p.connection.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) && closeErr == nil {
			closeErr = fmt.Errorf("close RabbitMQ connection: %w", err)
		}
	})

	return closeErr
}

var _ ports.MessagePublisher = (*Publisher)(nil)
