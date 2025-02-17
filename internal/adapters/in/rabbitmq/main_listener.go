package rabbitmq

import (
	"context"
	"fmt"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/in"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type CacheHitListener struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	useCase in.SlotGeneratorUseCase
	cfg     *config.Config
	logger  out.LoggerPort
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

func NewCacheHitListener(useCase in.SlotGeneratorUseCase, cfg *config.Config, logger out.LoggerPort) (*CacheHitListener, error) {
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

	return &CacheHitListener{
		conn:    conn,
		channel: channel,
		useCase: useCase,
		cfg:     cfg,
		logger:  logger,
	}, nil
}

func (l *CacheHitListener) Start(ctx context.Context) error {
	var err error
	err = l.startAppointmentQueue(ctx)
	if err != nil {
		return err
	}
	l.logger.Info("appointment.queue.started", out.LogFields{
		"queue": l.cfg.RabbitMq.QueueConfig.AppointmentQueueName,
	})
	err = l.startScheduleRuleQueue(ctx)
	if err != nil {
		return err
	}
	l.logger.Info("schedule_rule.queue.started", out.LogFields{
		"queue": l.cfg.RabbitMq.QueueConfig.ScheduleRuleQueueName,
	})
	err = l.startAllQueue(ctx)
	if err != nil {
		return err
	}
	l.logger.Info("_all_.queue.started", out.LogFields{
		"queue": l.cfg.RabbitMq.QueueConfig.AllQueueName,
	})
	err = l.startHealthcareServiceQueue(ctx)
	if err != nil {
		return err
	}
	l.logger.Info("healthcare_service.queue.started", out.LogFields{
		"queue": l.cfg.RabbitMq.QueueConfig.HealthcareServiceQueueName,
	})

	return nil
}

func (l *CacheHitListener) Stop() error {
	if l == nil || l.channel == nil {
		return nil
	}

	if err := l.channel.Close(); err != nil {
		return err
	}
	return l.conn.Close()
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
