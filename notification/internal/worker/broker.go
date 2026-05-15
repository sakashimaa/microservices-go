package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sakashimaa/billing-microservice/contracts/gen/auth_pb"
	"github.com/sakashimaa/billing-microservice/notification/internal/domain"
	"github.com/sakashimaa/billing-microservice/notification/internal/repository"
	"github.com/sakashimaa/billing-microservice/notification/internal/service"
)

var ErrFatal = errors.New("fatal error: do not retry")

type ConsumerWorker struct {
	conn       *amqp.Connection
	service    service.NotificationService
	authClient auth_pb.AuthServiceClient
	repo       repository.InboxRepository
	db         *pgxpool.Pool
}

func NewRabbitConsumer(
	conn *amqp.Connection,
	service service.NotificationService,
	authClient auth_pb.AuthServiceClient,
	repo repository.InboxRepository,
	db *pgxpool.Pool,
) *ConsumerWorker {
	return &ConsumerWorker{
		conn:       conn,
		service:    service,
		authClient: authClient,
		repo:       repo,
		db:         db,
	}
}

func (c *ConsumerWorker) StartConsume(ctx context.Context, queueName, routingKey string, exchangeNames ...string) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel for consumer: %w", err)
	}
	defer func() {
		if err := ch.Close(); err != nil {
			fmt.Printf("failed to close channel: %v\n", err)
			return
		}
	}()

	args := amqp.Table{
		"x-dead-letter-exchange": "notification.dlx",
	}
	q, err := ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		args,
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	for _, exchange := range exchangeNames {
		err = ch.QueueBind(q.Name, routingKey, exchange, false, nil)
		if err != nil {
			return fmt.Errorf("failed to bind queue to exchange %s: %w", exchange, err)
		}
		log.Printf("successfully bound queue %s to exchange %s\n", q.Name, exchange)
	}

	err = ch.Qos(
		10,
		0,
		false,
	)
	if err != nil {
		return fmt.Errorf("failed to set qos: %w", err)
	}

	err = ch.ExchangeDeclare(
		"dlx.events",
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare dlx exchange: %w", err)
	}

	dlq, err := ch.QueueDeclare("dlq.notification.events", true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to declare dlq: %w", err)
	}

	err = ch.QueueBind(dlq.Name, "", "dlx.events", false, nil)
	if err != nil {
		return fmt.Errorf("failed to bind dlq queue: %w", err)
	}

	msgs, err := ch.Consume(
		q.Name,
		"notification_worker_1",
		false,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}
	log.Printf("notification started listening on queue: %s\n", q.Name)

	for {
		select {
		case <-ctx.Done():
			log.Println("notification consumer worker stopped gracefully")
			return nil
		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("rabbit channel closed")
			}

			eventId := msg.MessageId
			if eventId == "" {
				return fmt.Errorf("no event id in message payload")
			}

			err := c.handleMessage(ctx, msg)
			if err != nil {
				log.Printf("failed to process event %s: %v\n", eventId, err)

				if errors.Is(err, ErrFatal) {
					log.Printf("fatal error occurred. Sending message %s to DLQ", eventId)
					if nackErr := msg.Nack(false, false); nackErr != nil {
						log.Printf("failed to nack message %s: %v\n", eventId, nackErr)
					}
				} else {
					if nackErr := msg.Nack(false, true); nackErr != nil {
						log.Printf("failed to nack message %s: %v\n", eventId, nackErr)
					}
				}

				continue
			}

			if err := msg.Ack(false); err != nil {
				log.Printf("failed to ack message %s:%v\n", msg.MessageId, err)
			}
		}
	}
}

func (c *ConsumerWorker) handleMessage(ctx context.Context, msg amqp.Delivery) error {
	eventID := msg.MessageId
	eventType := msg.RoutingKey

	if eventID == "" {
		return fmt.Errorf("empty event id")
	}

	var aggregateId string
	var aggregateType string

	switch eventType {
	case "AccountToppedUp":
		var payload DepositSuccessPayload
		if err := json.Unmarshal(msg.Body, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal: %v, %w", err, ErrFatal)
		}

		if payload.UserId == "" {
			return fmt.Errorf("empty user id in payload %w", ErrFatal)
		}

		aggregateId = payload.UserId
		aggregateType = "account"
	case "AccountRegistered":
		var payload AccountRegisteredPayload
		if err := json.Unmarshal(msg.Body, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal: %v, %w", err, ErrFatal)
		}

		aggregateId = payload.UserId
		aggregateType = "account"
	default:
		log.Printf("ignored unknown routing key: %s\n", msg.RoutingKey)
		return nil
	}

	tx, err := c.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		err = tx.Rollback(ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			log.Printf("failed to rollback transaction: %v\n", err)
		}
	}()

	err = c.repo.SaveProcessedMessageTx(ctx, tx, domain.SaveProcessedMessageParams{
		EventId:       eventID,
		AggregateId:   aggregateId,
		AggregateType: aggregateType,
		EventType:     eventType,
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicateMessage) {
			log.Printf("idempotency hit: message %s already processed. Skipping.", eventID)
			return nil
		}
		return fmt.Errorf("failed to save inbox: %w", err)
	}

	switch eventType {
	case "AccountToppedUp":
		var payload DepositSuccessPayload
		_ = json.Unmarshal(msg.Body, &payload)

		user, err := c.authClient.GetUserById(ctx, &auth_pb.GetUserByIdRequest{
			UserId: payload.UserId,
		})
		if err != nil {
			return fmt.Errorf("failed to get user by gRPC: %w", err)
		}

		err = c.service.SendDepositSuccessEmail(ctx, user.Email, payload.Amount)
		if err != nil {
			return fmt.Errorf("handleMessage: %w", err)
		}
	case "AccountRegistered":
		var payload AccountRegisteredPayload
		_ = json.Unmarshal(msg.Body, &payload)

		err = c.service.SendWelcomeEmail(ctx, payload.Email)
		if err != nil {
			return fmt.Errorf("handleMessage: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
