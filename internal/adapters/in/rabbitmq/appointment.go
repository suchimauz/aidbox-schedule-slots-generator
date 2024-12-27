package rabbitmq

import (
	"context"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type CacheAppointmentMessage struct {
	ScheduleID  string `json:"schedule_id"`
	Appointment domain.Appointment
}

func (l *CacheHitListener) startAppointmentQueue(ctx context.Context) error {
	queue, err := l.channel.QueueDeclare(
		l.cfg.RabbitMq.QueueConfig.AppointmentQueueName,
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
		l.cfg.RabbitMq.QueueConfig.AppointmentQueueBind,
		l.cfg.RabbitMq.QueueConfig.AppointmentQueueExchange,
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
				if err := l.processAppointmentMessage(ctx, msg); err != nil {
					msg.Nack(false, true) // requeue message
					continue
				}
				msg.Ack(false)
			}
		}
	}()

	return nil
}

func (l *CacheHitListener) processAppointmentMessage(ctx context.Context, msg amqp.Delivery) error {
	cacheMessageRoutingKey, err := l.parseCacheMessageRoutingKey(ctx, msg)
	if err != nil {
		return err
	}

	if cacheMessageRoutingKey.ResourceType != CacheHitResourceTypeAppointment {
		return nil
	}

	var msgJson CacheAppointmentMessage
	if err := json.Unmarshal(msg.Body, &msgJson); err != nil {
		return err
	}

	l.logger.Info("appointment.message.received", out.LogFields{
		"msg":       msgJson,
		"msgString": string(msg.Body),
	})

	if cacheMessageRoutingKey.CacheHitType == CacheHitTypeInvalidate {
		go l.useCase.InvalidateAppointmentCacheSlot(ctx, msgJson.ScheduleID, msgJson.Appointment)

		l.logger.Info("appointment.message.invalidated", out.LogFields{
			"appointment_id": msgJson.Appointment.ID,
		})
	}

	if cacheMessageRoutingKey.CacheHitType == CacheHitTypeStore {
		go l.useCase.StoreAppointmentCacheSlot(ctx, msgJson.ScheduleID, msgJson.Appointment)

		l.logger.Info("appointment.message.stored", out.LogFields{
			"appointment_id": msgJson.Appointment.ID,
		})
	}

	return nil
}
