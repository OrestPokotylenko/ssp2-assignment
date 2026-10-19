package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"southpark-messenger/services/api/internal/domain"
	"southpark-messenger/services/api/internal/ports"
)

const connectionTimeout = 3 * time.Second

type Publisher struct {
	url        string
	connection *amqp.Connection
	channel    *amqp.Channel
	queueName  string
	mu         sync.Mutex
	closeOnce  sync.Once
	closed     bool
}

func NewPublisher(url, queueName string) (*Publisher, error) {
	if url == "" {
		return nil, errors.New("RabbitMQ URL is required")
	}

	if queueName == "" {
		return nil, errors.New("RabbitMQ queue name is required")
	}

	publisher := &Publisher{url: url, queueName: queueName}
	if err := publisher.connectLocked(); err != nil {
		return nil, err
	}

	return publisher, nil
}

func (p *Publisher) connectLocked() error {
	connection, err := amqp.DialConfig(p.url, amqp.Config{
		Dial: amqp.DefaultDial(connectionTimeout),
	})
	if err != nil {
		return fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return fmt.Errorf("open RabbitMQ channel: %w", err)
	}

	_, err = channel.QueueDeclare(
		p.queueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return fmt.Errorf("declare RabbitMQ queue: %w", err)
	}

	if err := channel.Confirm(false); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return fmt.Errorf("enable RabbitMQ publisher confirms: %w", err)
	}

	p.connection = connection
	p.channel = channel
	return nil
}

func (p *Publisher) ensureConnectedLocked() error {
	if p.closed {
		return errors.New("RabbitMQ publisher is closed")
	}

	if p.connection != nil && !p.connection.IsClosed() && p.channel != nil && !p.channel.IsClosed() {
		return nil
	}

	_ = p.closeResourcesLocked()
	return p.connectLocked()
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

	if err := p.ensureConnectedLocked(); err != nil {
		return fmt.Errorf("restore RabbitMQ publisher: %w", err)
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
		// The delivery outcome is unknown after a connection-level failure, so
		// do not retry this message and risk a duplicate. The next request will
		// establish a fresh connection and channel.
		_ = p.closeResourcesLocked()
		return fmt.Errorf("publish to queue %q: %w", p.queueName, err)
	}

	acknowledged, err := confirmation.WaitContext(ctx)
	if err != nil {
		_ = p.closeResourcesLocked()
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
		p.closed = true
		closeErr = p.closeResourcesLocked()
	})

	return closeErr
}

func (p *Publisher) closeResourcesLocked() error {
	var closeErr error
	if p.channel != nil {
		if err := p.channel.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) {
			closeErr = fmt.Errorf("close RabbitMQ channel: %w", err)
		}
		p.channel = nil
	}

	if p.connection != nil {
		if err := p.connection.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) && closeErr == nil {
			closeErr = fmt.Errorf("close RabbitMQ connection: %w", err)
		}
		p.connection = nil
	}

	return closeErr
}

var _ ports.MessagePublisher = (*Publisher)(nil)
