package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type CacheAppointmentMessage struct {
	ScheduleID  string `json:"schedule_id"`
	Appointment domain.Appointment
}

func (l *CacheHitListener) startAppointmentQueue(ctx context.Context) error {
	// Проверяем контекст
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	l.logger.Info("rabbitmq.appointment.setup_starting", nil)

	// Объявляем обменник, если его нет
	exchangeName := l.cfg.RabbitMq.QueueConfig.AppointmentQueueExchange
	for attempts := 0; attempts < 3; attempts++ {
		err := l.channel.ExchangeDeclare(
			exchangeName, // имя обменника
			"topic",      // тип обменника
			true,         // durable
			false,        // auto-delete
			false,        // internal
			false,        // no-wait
			nil,          // аргументы
		)

		if err == nil {
			l.logger.Info("rabbitmq.exchange_declare.success", out.LogFields{
				"exchange": exchangeName,
			})
			break
		}

		l.logger.Warn("rabbitmq.exchange_declare.retry", out.LogFields{
			"exchange": exchangeName,
			"attempt":  attempts + 1,
			"error":    err.Error(),
		})

		if attempts == 2 {
			l.closeConnection(fmt.Sprintf("failed to declare exchange %s: %s", exchangeName, err.Error()))
			return fmt.Errorf("failed to declare exchange %s: %w", exchangeName, err)
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Объявляем очередь
	queueName := l.cfg.RabbitMq.QueueConfig.AppointmentQueueName
	var queue amqp.Queue
	var err error

	for attempts := 0; attempts < 3; attempts++ {
		queue, err = l.channel.QueueDeclare(
			queueName,
			true,  // durable
			true,  // delete when unused
			false, // exclusive
			false, // no-wait
			nil,   // arguments
		)

		if err == nil {
			l.logger.Info("rabbitmq.queue_declare.success", out.LogFields{
				"queue": queueName,
			})
			break
		}

		l.logger.Warn("rabbitmq.queue_declare.retry", out.LogFields{
			"queue":   queueName,
			"attempt": attempts + 1,
			"error":   err.Error(),
		})

		if attempts == 2 {
			l.closeConnection(fmt.Sprintf("failed to declare queue %s: %s", queueName, err.Error()))
			return fmt.Errorf("failed to declare queue %s: %w", queueName, err)
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Привязываем очередь к обменнику
	bindingKey := l.cfg.RabbitMq.QueueConfig.AppointmentQueueBind
	for attempts := 0; attempts < 3; attempts++ {
		err = l.channel.QueueBind(
			queue.Name,   // имя очереди
			bindingKey,   // ключ привязки
			exchangeName, // имя обменника
			false,        // no-wait
			nil,          // аргументы
		)

		if err == nil {
			l.logger.Info("rabbitmq.queue_bind.success", out.LogFields{
				"queue":    queue.Name,
				"binding":  bindingKey,
				"exchange": exchangeName,
			})
			break
		}

		l.logger.Warn("rabbitmq.queue_bind.retry", out.LogFields{
			"queue":    queue.Name,
			"binding":  bindingKey,
			"exchange": exchangeName,
			"attempt":  attempts + 1,
			"error":    err.Error(),
		})

		if attempts == 2 {
			l.closeConnection(fmt.Sprintf("failed to bind queue %s: %s", queue.Name, err.Error()))
			return fmt.Errorf("failed to bind queue %s: %w", queue.Name, err)
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Настраиваем потребителя
	var msgs <-chan amqp.Delivery
	consumerID := fmt.Sprintf("consumer-%s-%d", queue.Name, time.Now().UnixNano())

	for attempts := 0; attempts < 3; attempts++ {
		msgs, err = l.channel.Consume(
			queue.Name,
			consumerID, // уникальный ID
			false,      // auto-ack (изменено на false для ручного подтверждения)
			false,      // exclusive
			false,      // no-local
			false,      // no-wait
			nil,        // args
		)

		if err == nil {
			l.logger.Info("rabbitmq.consume.success", out.LogFields{
				"queue":      queue.Name,
				"consumerID": consumerID,
			})
			break
		}

		l.logger.Warn("rabbitmq.consume.retry", out.LogFields{
			"queue":      queue.Name,
			"consumerID": consumerID,
			"attempt":    attempts + 1,
			"error":      err.Error(),
		})

		if attempts == 2 {
			l.closeConnection(fmt.Sprintf("failed to consume from queue %s: %s", queue.Name, err.Error()))
			return fmt.Errorf("failed to consume from queue %s: %w", queue.Name, err)
		}

		time.Sleep(500 * time.Millisecond)
	}

	l.logger.Info("rabbitmq.queue.started", out.LogFields{
		"queue":    queue.Name,
		"binding":  bindingKey,
		"exchange": exchangeName,
	})

	// Создаем канал отмены для консьюмера
	consumerCancel := make(chan struct{})
	l.addConsumerCancel(consumerCancel)

	// Увеличиваем счетчик горутин
	l.consumerWg.Add(1)

	// Запускаем обработку сообщений в отдельной горутине
	go func() {
		defer l.consumerWg.Done()
		l.logger.Info("rabbitmq.consumer.started", out.LogFields{
			"queue":      queue.Name,
			"consumerID": consumerID,
		})

		for {
			select {
			case <-ctx.Done():
				l.logger.Info("rabbitmq.consumer.stopping_by_context", out.LogFields{
					"queue":      queue.Name,
					"consumerID": consumerID,
				})
				return
			case <-consumerCancel:
				l.logger.Info("rabbitmq.consumer.stopping_by_cancel", out.LogFields{
					"queue":      queue.Name,
					"consumerID": consumerID,
				})
				return
			case msg, ok := <-msgs:
				if !ok {
					l.logger.Warn("rabbitmq.consumer.channel_closed", out.LogFields{
						"queue":      queue.Name,
						"consumerID": consumerID,
					})
					// Канал закрыт, закрываем соединение для переподключения
					l.closeConnection(fmt.Sprintf("consumer channel closed for queue %s", queue.Name))
					return
				}

				l.logger.Debug("rabbitmq.message.received", out.LogFields{
					"queue":      queue.Name,
					"routingKey": msg.RoutingKey,
					"messageId":  msg.MessageId,
				})

				// Обрабатываем сообщение
				err := l.processAppointmentMessage(ctx, msg)

				// Подтверждаем получение сообщения только после успешной обработки
				if err != nil {
					l.logger.Error("rabbitmq.process_message.failed", out.LogFields{
						"queue":      queue.Name,
						"routingKey": msg.RoutingKey,
						"messageId":  msg.MessageId,
						"error":      err.Error(),
					})

					// Отклоняем сообщение при ошибке, но не возвращаем в очередь
					if err := msg.Nack(false, false); err != nil {
						l.logger.Error("rabbitmq.message.nack_failed", out.LogFields{
							"error": err.Error(),
						})
					}
				} else {
					// Подтверждаем успешную обработку
					if err := msg.Ack(false); err != nil {
						l.logger.Error("rabbitmq.message.ack_failed", out.LogFields{
							"error": err.Error(),
						})
					}
				}
			}
		}
	}()

	return nil
}

func (l *CacheHitListener) processAppointmentMessage(ctx context.Context, msg amqp.Delivery) error {
	l.logger.Debug("rabbitmq.processing_message", out.LogFields{
		"routingKey": msg.RoutingKey,
		"body":       string(msg.Body),
	})

	cacheMessageRoutingKey, err := l.parseCacheMessageRoutingKey(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to parse routing key: %w", err)
	}

	if cacheMessageRoutingKey.ResourceType != CacheHitResourceTypeAppointment {
		l.logger.Debug("rabbitmq.message.skipped", out.LogFields{
			"expected": string(CacheHitResourceTypeAppointment),
			"actual":   string(cacheMessageRoutingKey.ResourceType),
		})
		return nil
	}

	var msgJson CacheAppointmentMessage
	if err := json.Unmarshal(msg.Body, &msgJson); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	l.logger.Info("appointment.message.received", out.LogFields{
		"appointment_id": msgJson.Appointment.ID,
		"schedule_id":    msgJson.ScheduleID,
		"cacheHitType":   string(cacheMessageRoutingKey.CacheHitType),
	})

	// Создаем контекст с таймаутом для операций
	opCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if cacheMessageRoutingKey.CacheHitType == CacheHitTypeInvalidate {
		if err := l.useCase.InvalidateAppointmentCacheSlot(opCtx, msgJson.ScheduleID, msgJson.Appointment); err != nil {
			l.logger.Error("appointment.invalidate_cache_slot.failed", out.LogFields{
				"appointment_id": msgJson.Appointment.ID,
				"schedule_id":    msgJson.ScheduleID,
				"error":          err.Error(),
			})
			return err
		}

		l.logger.Info("appointment.message.invalidated", out.LogFields{
			"appointment_id": msgJson.Appointment.ID,
		})
	}

	if cacheMessageRoutingKey.CacheHitType == CacheHitTypeStore {
		if err := l.useCase.StoreAppointmentCacheSlot(opCtx, msgJson.ScheduleID, msgJson.Appointment); err != nil {
			l.logger.Error("appointment.store_cache_slot.failed", out.LogFields{
				"appointment_id": msgJson.Appointment.ID,
				"schedule_id":    msgJson.ScheduleID,
				"error":          err.Error(),
			})
			return err
		}

		l.logger.Info("appointment.message.stored", out.LogFields{
			"appointment_id": msgJson.Appointment.ID,
		})
	}

	return nil
}
