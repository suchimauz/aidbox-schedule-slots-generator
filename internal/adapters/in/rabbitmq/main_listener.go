package rabbitmq

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/in"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type CacheHitListener struct {
	conn            *amqp.Connection
	channel         *amqp.Channel
	useCase         in.SlotGeneratorUseCase
	cfg             *config.Config
	logger          out.LoggerPort
	reconnectDelay  time.Duration
	done            chan struct{}
	notifyConnClose chan *amqp.Error
	notifyChanClose chan *amqp.Error
	isConnected     bool
	mu              sync.Mutex
	consumerCancels []chan struct{}
	consumerWg      sync.WaitGroup
	cancelMu        sync.Mutex
}

type (
	CacheHitType         string
	CacheHitResourceType string
)

type CacheMessageRoutingKey struct {
	Source       string
	Receiver     string
	ResourceType CacheHitResourceType
	CacheHitType CacheHitType
}

const (
	CacheHitResourceTypeAll               CacheHitResourceType = "_all_"
	CacheHitResourceTypeScheduleRule      CacheHitResourceType = "schedulerule"
	CacheHitResourceTypeAppointment       CacheHitResourceType = "appointment"
	CacheHitResourceTypeHealthcareService CacheHitResourceType = "healthcareservice"
)

const (
	CacheHitTypeStore      CacheHitType = "store"
	CacheHitTypeInvalidate CacheHitType = "invalidate"
)

func NewCacheHitListener(cfg *config.Config, useCase in.SlotGeneratorUseCase, logger out.LoggerPort) (*CacheHitListener, error) {
	listener := &CacheHitListener{
		useCase:         useCase,
		cfg:             cfg,
		logger:          logger.WithModule("CacheHitListener"),
		reconnectDelay:  5 * time.Second,
		done:            make(chan struct{}),
		isConnected:     false,
		consumerCancels: make([]chan struct{}, 0),
	}

	return listener, nil
}

// connect устанавливает соединение с RabbitMQ
func (l *CacheHitListener) connect() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.isConnected {
		return nil
	}

	l.logger.Info("rabbitmq.connecting", nil)

	// Подключаемся к RabbitMQ
	conn, err := amqp.Dial(l.cfg.RabbitMq.AmqpUri)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	l.conn = conn
	l.notifyConnClose = make(chan *amqp.Error)
	l.conn.NotifyClose(l.notifyConnClose)

	// Создаем канал
	channel, err := conn.Channel()
	if err != nil {
		l.conn.Close()
		return fmt.Errorf("failed to open a channel: %w", err)
	}

	l.channel = channel
	l.notifyChanClose = make(chan *amqp.Error)
	l.channel.NotifyClose(l.notifyChanClose)

	// Устанавливаем QoS
	err = l.channel.Qos(
		10,    // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		l.channel.Close()
		l.conn.Close()
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	l.isConnected = true
	l.logger.Info("rabbitmq.connected", nil)

	return nil
}

// handleReconnect обрабатывает переподключение к RabbitMQ
func (l *CacheHitListener) handleReconnect() {
	for {
		l.mu.Lock()
		if !l.isConnected {
			l.logger.Info("rabbitmq.reconnecting", out.LogFields{
				"delay": l.reconnectDelay.String(),
			})

			// Ждем перед повторной попыткой
			l.mu.Unlock()
			time.Sleep(l.reconnectDelay)
			l.mu.Lock()

			// Пытаемся подключиться
			conn, err := amqp.Dial(l.cfg.RabbitMq.AmqpUri)
			if err != nil {
				l.logger.Error("rabbitmq.reconnect.failed", out.LogFields{
					"error": err.Error(),
				})
				l.mu.Unlock()
				continue
			}

			l.conn = conn
			l.notifyConnClose = make(chan *amqp.Error)
			l.conn.NotifyClose(l.notifyConnClose)

			// Создаем новый канал
			channel, err := conn.Channel()
			if err != nil {
				l.logger.Error("rabbitmq.channel.failed", out.LogFields{
					"error": err.Error(),
				})
				l.conn.Close()
				l.mu.Unlock()
				continue
			}

			l.channel = channel
			l.notifyChanClose = make(chan *amqp.Error)
			l.channel.NotifyClose(l.notifyChanClose)

			// Устанавливаем QoS с фиксированным значением
			err = l.channel.Qos(
				10,    // prefetch count - устанавливаем значение по умолчанию
				0,     // prefetch size
				false, // global
			)
			if err != nil {
				l.logger.Error("rabbitmq.qos.failed", out.LogFields{
					"error": err.Error(),
				})
				l.channel.Close()
				l.conn.Close()
				l.mu.Unlock()
				continue
			}

			// Успешно подключились
			l.isConnected = true

			// Включаем кэш после успешного подключения
			l.cfg.Cache.Enabled = true

			l.logger.Info("rabbitmq.reconnected", out.LogFields{
				"cache_enabled": l.cfg.Cache.Enabled,
			})

			// Запускаем все очереди
			go func() {
				ctx := context.Background()
				if err := l.setupQueues(ctx); err != nil {
					l.logger.Error("rabbitmq.start_queues.failed", out.LogFields{
						"error": err.Error(),
					})
					l.closeConnection("failed to start queues after reconnect")
				}
			}()

			l.mu.Unlock()
			break
		}
		l.mu.Unlock()

		// Если уже подключены, просто выходим из цикла
		break
	}
}

// setupQueues создает все необходимые очереди и привязки
func (l *CacheHitListener) setupQueues(ctx context.Context) error {
	if err := l.startAllQueue(ctx); err != nil {
		return fmt.Errorf("failed to start all queue: %w", err)
	}

	if err := l.startScheduleRuleQueue(ctx); err != nil {
		return fmt.Errorf("failed to start schedule rule queue: %w", err)
	}

	if err := l.startHealthcareServiceQueue(ctx); err != nil {
		return fmt.Errorf("failed to start healthcare service queue: %w", err)
	}

	if err := l.startAppointmentQueue(ctx); err != nil {
		return fmt.Errorf("failed to start appointment queue: %w", err)
	}

	return nil
}

// invalidateAllCache инвалидирует весь кэш
func (l *CacheHitListener) invalidateAllCache(ctx context.Context) {
	l.logger.Info("rabbitmq.connection_lost.invalidating_cache", nil)

	// Инвалидируем кэш
	if err := l.useCase.InvalidateAllSlotsCache(ctx); err != nil {
		l.logger.Error("rabbitmq.invalidate_slots_cache.failed", out.LogFields{
			"error": err.Error(),
		})
	} else {
		l.logger.Info("rabbitmq.invalidate_slots_cache.success", nil)
	}

	if err := l.useCase.InvalidateScheduleRuleGlobalCache(ctx); err != nil {
		l.logger.Error("rabbitmq.invalidate_schedule_rule_global_cache.failed", out.LogFields{
			"error": err.Error(),
		})
	} else {
		l.logger.Info("rabbitmq.invalidate_schedule_rule_global_cache.success", nil)
	}

	if err := l.useCase.InvalidateAllScheduleRuleCache(ctx); err != nil {
		l.logger.Error("rabbitmq.invalidate_all_schedule_rule_cache.failed", out.LogFields{
			"error": err.Error(),
		})
	} else {
		l.logger.Info("rabbitmq.invalidate_all_schedule_rule_cache.success", nil)
	}

	if err := l.useCase.InvalidateAllHealthcareServiceCache(ctx); err != nil {
		l.logger.Error("rabbitmq.invalidate_all_healthcare_service_cache.failed", out.LogFields{
			"error": err.Error(),
		})
	} else {
		l.logger.Info("rabbitmq.invalidate_all_healthcare_service_cache.success", nil)
	}
}

// Close закрывает соединение с RabbitMQ
func (l *CacheHitListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.isConnected {
		return nil
	}

	close(l.done)

	if err := l.channel.Close(); err != nil {
		return err
	}

	if err := l.conn.Close(); err != nil {
		return err
	}

	l.isConnected = false
	l.logger.Info("rabbitmq.connection_closed", nil)

	return nil
}

// Start запускает слушателя RabbitMQ
func (l *CacheHitListener) Start(ctx context.Context) error {
	l.done = make(chan struct{})

	// Первоначальное подключение
	if err := l.connect(); err != nil {
		return err
	}

	// Запускаем все очереди
	if err := l.setupQueues(ctx); err != nil {
		l.closeConnection("failed to start queues")
		return err
	}

	// Запускаем горутину для мониторинга соединения
	go l.monitorConnection(ctx)

	return nil
}

// Новый метод для мониторинга соединения
func (l *CacheHitListener) monitorConnection(ctx context.Context) {
	for {
		select {
		case <-l.done:
			l.logger.Info("rabbitmq.monitor.stopping", nil)
			return
		case <-ctx.Done():
			l.logger.Info("rabbitmq.monitor.context_done", nil)
			l.Stop()
			return
		case err := <-l.notifyConnClose:
			if err != nil {
				l.logger.Error("rabbitmq.connection.closed_with_error", out.LogFields{
					"error": err.Error(),
				})
			} else {
				l.logger.Warn("rabbitmq.connection.closed", nil)
			}

			// Закрываем соединение и переподключаемся
			l.closeConnection("connection closed by server")
			l.handleReconnect()
		case err := <-l.notifyChanClose:
			if err != nil {
				l.logger.Error("rabbitmq.channel.closed_with_error", out.LogFields{
					"error": err.Error(),
				})
			} else {
				l.logger.Warn("rabbitmq.channel.closed", nil)
			}

			// Закрываем соединение и переподключаемся
			l.closeConnection("channel closed by server")
			l.handleReconnect()
		}
	}
}

// Stop останавливает слушателя RabbitMQ
func (l *CacheHitListener) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.isConnected {
		return
	}

	l.logger.Info("rabbitmq.stopping", nil)

	// Сигнализируем о завершении работы
	close(l.done)

	// Закрываем все консьюмеры
	l.closeConsumers()

	// Ждем завершения всех горутин обработки сообщений
	waitCh := make(chan struct{})
	go func() {
		l.consumerWg.Wait()
		close(waitCh)
	}()

	// Ждем завершения горутин с таймаутом
	select {
	case <-waitCh:
		l.logger.Info("rabbitmq.all_consumers_stopped", nil)
	case <-time.After(5 * time.Second):
		l.logger.Warn("rabbitmq.consumers_stop_timeout", nil)
	}

	// Закрываем канал и соединение
	if l.channel != nil {
		l.channel.Close()
	}

	if l.conn != nil {
		l.conn.Close()
	}

	l.isConnected = false
	l.logger.Info("rabbitmq.stopped", nil)
}

// Пример routingKey:
// aidbox.slot-generator-svc.schedulerule.store
// aidbox.slot-generator-svc.scheduleruleglobal.invalidate
// aidbox.slot-generator-svc.appointment.store
// aidbox.slot-generator-svc.appointment.invalidate
func (l *CacheHitListener) parseCacheMessageRoutingKey(ctx context.Context, msg amqp.Delivery) (CacheMessageRoutingKey, error) {
	routingKey := msg.RoutingKey
	parts := strings.Split(routingKey, ".")

	if len(parts) < 5 {
		return CacheMessageRoutingKey{}, fmt.Errorf("invalid routing key: %s", routingKey)
	}

	return CacheMessageRoutingKey{
		Source:       parts[0],
		Receiver:     parts[1],
		ResourceType: CacheHitResourceType(parts[2]),
		CacheHitType: CacheHitType(parts[4]),
	}, nil
}

// closeConsumers закрывает все активные консьюмеры и ждет их завершения
func (l *CacheHitListener) closeConsumers() {
	l.cancelMu.Lock()
	defer l.cancelMu.Unlock()

	l.logger.Info("rabbitmq.closing_consumers", out.LogFields{
		"count": len(l.consumerCancels),
	})

	// Отправляем сигнал отмены всем консьюмерам
	for _, cancel := range l.consumerCancels {
		close(cancel)
	}

	// Очищаем список каналов отмены
	l.consumerCancels = nil
}

// addConsumerCancel добавляет канал отмены консьюмера
func (l *CacheHitListener) addConsumerCancel(cancel chan struct{}) {
	l.cancelMu.Lock()
	defer l.cancelMu.Unlock()

	l.consumerCancels = append(l.consumerCancels, cancel)
}

// closeConnection закрывает соединение с RabbitMQ, что вызовет процесс переподключения
func (l *CacheHitListener) closeConnection(reason string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.isConnected {
		return
	}

	l.isConnected = false

	// Выключаем кэш на время переподключения
	l.cfg.Cache.Enabled = false

	l.logger.Warn("rabbitmq.force_disconnect", out.LogFields{
		"reason": reason,
	})

	// Инвалидируем весь кэш перед закрытием соединения
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	l.invalidateAllCache(ctx)

	// Закрываем все консьюмеры перед закрытием соединения
	l.closeConsumers()

	// Ждем завершения всех горутин обработки сообщений
	waitCh := make(chan struct{})
	go func() {
		l.consumerWg.Wait()
		close(waitCh)
	}()

	// Ждем завершения горутин с таймаутом
	select {
	case <-waitCh:
		l.logger.Info("rabbitmq.all_consumers_stopped", nil)
	case <-time.After(5 * time.Second):
		l.logger.Warn("rabbitmq.consumers_stop_timeout", nil)
	}

	// Закрываем канал и соединение
	if l.channel != nil {
		// Пытаемся удалить все бинды перед закрытием
		l.removeAllBindings()

		if err := l.channel.Close(); err != nil {
			l.logger.Error("rabbitmq.channel_close.failed", out.LogFields{
				"error": err.Error(),
			})
		}
		l.channel = nil
	}

	if l.conn != nil {
		if err := l.conn.Close(); err != nil {
			l.logger.Error("rabbitmq.connection_close.failed", out.LogFields{
				"error": err.Error(),
			})
		}
		l.conn = nil
	}

	l.logger.Info("rabbitmq.connection_closed", out.LogFields{
		"reason": reason,
	})

	// Соединение будет восстановлено через handleReconnect
}

// Добавим новый метод для удаления всех привязок
func (l *CacheHitListener) removeAllBindings() {
	// Список всех очередей и их привязок
	bindings := []struct {
		queueName    string
		bindingKey   string
		exchangeName string
	}{
		{
			queueName:    l.cfg.RabbitMq.QueueConfig.ScheduleRuleQueueName,
			bindingKey:   l.cfg.RabbitMq.QueueConfig.ScheduleRuleQueueBind,
			exchangeName: l.cfg.RabbitMq.QueueConfig.ScheduleRuleQueueExchange,
		},
		{
			queueName:    l.cfg.RabbitMq.QueueConfig.HealthcareServiceQueueName,
			bindingKey:   l.cfg.RabbitMq.QueueConfig.HealthcareServiceQueueBind,
			exchangeName: l.cfg.RabbitMq.QueueConfig.HealthcareServiceQueueExchange,
		},
		{
			queueName:    l.cfg.RabbitMq.QueueConfig.AppointmentQueueName,
			bindingKey:   l.cfg.RabbitMq.QueueConfig.AppointmentQueueBind,
			exchangeName: l.cfg.RabbitMq.QueueConfig.AppointmentQueueExchange,
		},
		{
			queueName:    l.cfg.RabbitMq.QueueConfig.AllQueueName,
			bindingKey:   l.cfg.RabbitMq.QueueConfig.AllQueueBind,
			exchangeName: l.cfg.RabbitMq.QueueConfig.AllQueueExchange,
		},
	}

	// Пытаемся удалить каждую привязку
	for _, binding := range bindings {
		err := l.channel.QueueUnbind(
			binding.queueName,
			binding.bindingKey,
			binding.exchangeName,
			nil, // аргументы
		)

		if err != nil {
			l.logger.Warn("rabbitmq.queue_unbind.failed", out.LogFields{
				"queue":    binding.queueName,
				"binding":  binding.bindingKey,
				"exchange": binding.exchangeName,
				"error":    err.Error(),
			})
		} else {
			l.logger.Info("rabbitmq.queue_unbind.success", out.LogFields{
				"queue":    binding.queueName,
				"binding":  binding.bindingKey,
				"exchange": binding.exchangeName,
			})
		}
	}
}
