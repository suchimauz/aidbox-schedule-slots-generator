package rabbitmq

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/in"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type AppointmentListener struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	useCase in.SlotGeneratorUseCase
	cfg     *config.Config
	logger  out.LoggerPort
}

type CacheType string

const (
	CacheTypeStore      CacheType = "store"
	CacheTypeInvalidate CacheType = "invalidate"
)

type AppointmentCacheUpdateMessage domain.Appointment

func NewAppointmentListener(useCase in.SlotGeneratorUseCase, cfg *config.Config, logger out.LoggerPort) (*AppointmentListener, error) {
	if !cfg.RabbitMq.Enabled {
		logger.Info("rabbitmq.disabled", out.LogFields{
			"message": "RabbitMQ is disabled, listener will not be started",
		})
		return nil, nil
	}

	conn, err := amqp.Dial(cfg.RabbitMq.AmqpUri)
	if err != nil {
		logger.Error("rabbitmq.connect.failed", out.LogFields{
			"error": err.Error(),
			"url":   cfg.RabbitMq.AmqpUri,
		})
		return nil, err
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		logger.Error("rabbitmq.channel.failed", out.LogFields{
			"error": err.Error(),
		})
		return nil, err
	}

	return &AppointmentListener{
		conn:    conn,
		channel: channel,
		useCase: useCase,
		cfg:     cfg,
		logger:  logger,
	}, nil
}

func (l *AppointmentListener) Start(ctx context.Context) error {
	// queue, err := l.channel.QueueDeclare(
	// 	l.cfg.RabbitMq.QueueConfig.AppointmentQueueName,
	// 	true,  // durable
	// 	false, // delete when unused
	// 	false, // exclusive
	// 	false, // no-wait
	// 	nil,   // arguments
	// )
	// if err != nil {
	// 	return err
	// }

	// msgs, err := l.channel.Consume(
	// 	queue.Name,
	// 	"",    // consumer
	// 	false, // auto-ack
	// 	false, // exclusive
	// 	false, // no-local
	// 	false, // no-wait
	// 	nil,   // args
	// )
	// if err != nil {
	// 	return err
	// }

	// go func() {
	// 	for {
	// 		select {
	// 		case <-ctx.Done():
	// 			return
	// 		case msg := <-msgs:
	// 			if err := l.processMessage(ctx, msg); err != nil {
	// 				log.Printf("Error processing message: %v", err)
	// 				msg.Nack(false, true) // requeue message
	// 				continue
	// 			}
	// 			msg.Ack(false)
	// 		}
	// 	}
	// }()

	return nil
}

func (l *AppointmentListener) processMessage(ctx context.Context, msg amqp.Delivery) error {
	// var update AppointmentUpdate
	// if err := json.Unmarshal(msg.Body, &update); err != nil {
	// 	return err
	// }

	// return l.useCase.UpdateSlotStatus(ctx, update.AppointmentID)
	return nil
}

func (l *AppointmentListener) Stop() error {
	if l == nil || l.channel == nil {
		return nil
	}

	if err := l.channel.Close(); err != nil {
		return err
	}
	return l.conn.Close()
}
