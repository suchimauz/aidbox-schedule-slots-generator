package rabbitmq

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

func (l *CacheHitListener) startAllQueue(ctx context.Context) error {
	queue, err := l.channel.QueueDeclare(
		l.cfg.RabbitMq.QueueConfig.AllQueueName,
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
		l.cfg.RabbitMq.QueueConfig.AllQueueBind,
		l.cfg.RabbitMq.QueueConfig.AllQueueExchange,
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
				if err := l.processAllMessage(ctx, msg); err != nil {
					msg.Nack(false, true) // requeue message
					continue
				}
				msg.Ack(false)
			}
		}
	}()

	return nil
}

func (l *CacheHitListener) processAllMessage(ctx context.Context, msg amqp.Delivery) error {
	cacheMessageRoutingKey, err := l.parseCacheMessageRoutingKey(ctx, msg)
	if err != nil {
		return err
	}

	if cacheMessageRoutingKey.ResourceType != CacheHitResourceTypeAll {
		return nil
	}

	// Если поменялось глобальное расписание, то нужно очистить весь кэш слотов и самого глобального расписания
	if cacheMessageRoutingKey.CacheHitType == CacheHitTypeInvalidate {
		go l.useCase.InvalidateAllSlotsCache(ctx)
		go l.useCase.InvalidateScheduleRuleGlobalCache(ctx)
		go l.useCase.InvalidateAllScheduleRuleCache(ctx)
		go l.useCase.InvalidateAllHealthcareServiceCache(ctx)

		l.logger.Info("_all_.message.invalidated", out.LogFields{
			"slots_cache":                true,
			"schedule_rule_global_cache": true,
			"schedule_rule_cache":        true,
			"healthcare_service_cache":   true,
		})
	}

	return nil
}
