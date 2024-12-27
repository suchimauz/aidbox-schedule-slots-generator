package rabbitmq

import (
	"context"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type CacheScheduleRuleMessage struct {
	ID           string `json:"id"`
	ResourceType string `json:"resourceType"`
}

func (l *CacheHitListener) startScheduleRuleQueue(ctx context.Context) error {
	queue, err := l.channel.QueueDeclare(
		l.cfg.RabbitMq.QueueConfig.ScheduleRuleQueueName,
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
		l.cfg.RabbitMq.QueueConfig.ScheduleRuleQueueBind,
		l.cfg.RabbitMq.QueueConfig.ScheduleRuleQueueExchange,
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
				if err := l.processScheduleRuleMessage(ctx, msg); err != nil {
					msg.Nack(false, true) // requeue message
					continue
				}
				msg.Ack(false)
			}
		}
	}()

	return nil
}

func (l *CacheHitListener) processScheduleRuleMessage(ctx context.Context, msg amqp.Delivery) error {
	cacheMessageRoutingKey, err := l.parseCacheMessageRoutingKey(ctx, msg)
	if err != nil {
		return err
	}

	if cacheMessageRoutingKey.ResourceType != CacheHitResourceTypeScheduleRule {
		return nil
	}

	var msgJson CacheScheduleRuleMessage
	if err := json.Unmarshal(msg.Body, &msgJson); err != nil {
		return err
	}

	// Если поменялось расписание, то нужно очистить кэш слотов для этого расписания и кэш самого расписания
	if cacheMessageRoutingKey.CacheHitType == CacheHitTypeInvalidate {
		go l.useCase.InvalidateSlotsCache(ctx, msgJson.ID)
		go l.useCase.InvalidateScheduleRuleCache(ctx, msgJson.ID)

		l.logger.Info("schedule_rule.message.invalidated", out.LogFields{
			"schedule_rule_id": msgJson.ID,
		})
	}

	return nil
}
