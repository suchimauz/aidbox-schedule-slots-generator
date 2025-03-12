package rabbitmq

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

func (l *CacheHitListener) startAllQueue(ctx context.Context) error {
	// Проверяем контекст
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	l.logger.Info("rabbitmq.all.setup_starting", nil)

	// Объявляем обменник, если его нет
	exchangeName := l.cfg.RabbitMq.QueueConfig.AllQueueExchange
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
	queueName := l.cfg.RabbitMq.QueueConfig.AllQueueName
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
	bindingKey := l.cfg.RabbitMq.QueueConfig.AllQueueBind
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
				err := l.processAllMessage(ctx, msg)

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

func (l *CacheHitListener) processAllMessage(ctx context.Context, msg amqp.Delivery) error {
	l.logger.Debug("rabbitmq.processing_message", out.LogFields{
		"routingKey": msg.RoutingKey,
		"body":       string(msg.Body),
	})

	cacheMessageRoutingKey, err := l.parseCacheMessageRoutingKey(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to parse routing key: %w", err)
	}

	if cacheMessageRoutingKey.ResourceType != CacheHitResourceTypeAll {
		l.logger.Debug("rabbitmq.message.skipped", out.LogFields{
			"expected": string(CacheHitResourceTypeAll),
			"actual":   string(cacheMessageRoutingKey.ResourceType),
		})
		return nil
	}

	l.logger.Info("_all_.message.received", out.LogFields{
		"cacheHitType": string(cacheMessageRoutingKey.CacheHitType),
	})

	// Если поменялось глобальное расписание, то нужно очистить весь кэш слотов и самого глобального расписания
	if cacheMessageRoutingKey.CacheHitType == CacheHitTypeInvalidate {
		// Создаем контекст с таймаутом для операций инвалидации
		invalidateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Выполняем операции инвалидации последовательно
		if err := l.useCase.InvalidateAllSlotsCache(invalidateCtx); err != nil {
			l.logger.Error("_all_.invalidate_all_slots_cache.failed", out.LogFields{
				"error": err.Error(),
			})
		}

		if err := l.useCase.InvalidateScheduleRuleGlobalCache(invalidateCtx); err != nil {
			l.logger.Error("_all_.invalidate_schedule_rule_global_cache.failed", out.LogFields{
				"error": err.Error(),
			})
		}

		if err := l.useCase.InvalidateAllScheduleRuleCache(invalidateCtx); err != nil {
			l.logger.Error("_all_.invalidate_all_schedule_rule_cache.failed", out.LogFields{
				"error": err.Error(),
			})
		}

		if err := l.useCase.InvalidateAllHealthcareServiceCache(invalidateCtx); err != nil {
			l.logger.Error("_all_.invalidate_all_healthcare_service_cache.failed", out.LogFields{
				"error": err.Error(),
			})
		}

		l.logger.Info("_all_.message.invalidated", out.LogFields{
			"slots_cache":                true,
			"schedule_rule_global_cache": true,
			"schedule_rule_cache":        true,
			"healthcare_service_cache":   true,
		})
	}

	return nil
}
