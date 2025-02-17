package rabbitmq

import (
	"context"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type CacheHealthcareServiceMessage struct {
	ID           string `json:"id"`
	ResourceType string `json:"resourceType"`
}

func (l *CacheHitListener) startHealthcareServiceQueue(ctx context.Context) error {
	queue, err := l.channel.QueueDeclare(
		l.cfg.RabbitMq.QueueConfig.HealthcareServiceQueueName,
		true,  // durable
		true,  // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return err
	}
	err = l.channel.QueueBind(
		queue.Name,
		l.cfg.RabbitMq.QueueConfig.HealthcareServiceQueueBind,
		l.cfg.RabbitMq.QueueConfig.HealthcareServiceQueueExchange,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	msgs, err := l.channel.Consume(
		queue.Name,
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-msgs:
				if err := l.processHealthcareServiceMessage(ctx, msg); err != nil {
					msg.Nack(false, true) // requeue message
					continue
				}
				msg.Ack(false)
			}
		}
	}()

	return nil
}

func (l *CacheHitListener) processHealthcareServiceMessage(ctx context.Context, msg amqp.Delivery) error {
	cacheMessageRoutingKey, err := l.parseCacheMessageRoutingKey(ctx, msg)
	if err != nil {
		return err
	}

	if cacheMessageRoutingKey.ResourceType != CacheHitResourceTypeHealthcareService {
		return nil
	}

	var msgJson CacheHealthcareServiceMessage
	if err := json.Unmarshal(msg.Body, &msgJson); err != nil {
		return err
	}

	// Если поменялось расписание, то нужно очистить кэш слотов для этого расписания и кэш самого расписания
	if cacheMessageRoutingKey.CacheHitType == CacheHitTypeInvalidate {
		go l.useCase.InvalidateSlotsCache(ctx, msgJson.ID)
		go l.useCase.InvalidateHealthcareServiceCache(ctx, msgJson.ID)

		l.logger.Info("healthcare_service.message.invalidated", out.LogFields{
			"healthcare_service_id": msgJson.ID,
		})
	}

	return nil
}
