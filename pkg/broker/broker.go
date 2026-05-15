package broker

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sakashimaa/billing-microservice/pkg/outbox"
)

type RabbitPublisher struct {
	channel  *amqp.Channel
	exchange string
}

func NewRabbitPublisher(channel *amqp.Channel, exchange string) outbox.EventPublisher {
	return &RabbitPublisher{
		channel:  channel,
		exchange: exchange,
	}
}

func (p *RabbitPublisher) Publish(ctx context.Context, event *outbox.Event) error {
	body, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	err = p.channel.PublishWithContext(
		ctx,
		p.exchange,
		event.EventType,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			MessageId:    fmt.Sprintf("%d", event.Id),
			DeliveryMode: amqp.Persistent,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish to rabbitmq: %w", err)
	}
	return nil
}
