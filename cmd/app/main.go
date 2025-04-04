package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/adapters/in/http"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/adapters/in/rabbitmq"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/adapters/out/aidbox"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/adapters/out/cache"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/adapters/out/logger"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/services"
)

func main() {
	// Загрузка конфигурации
	cfg, err := config.NewConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Инициализация логгера с таймзоной
	mainLogger, err := logger.NewConsoleLogger(cfg.App.Timezone)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	logger := mainLogger.WithModule("Main")

	logger.Info("app.starting", out.LogFields{
		"ampq_uri":        cfg.RabbitMq.AmqpUri,
		"version":         cfg.App.Version,
		"env":             cfg.App.Env,
		"timezone":        cfg.App.Timezone,
		"rabbitMqQueue":   cfg.RabbitMq.QueueConfig,
		"rabbitMqEnabled": cfg.RabbitMq.Enabled,
		"cache":           cfg.Cache,
	})

	// Настройка Gin в зависимости от окружения
	if cfg.IsNotLocal() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Инициализация адаптеров
	aidboxAdapter := aidbox.NewAidboxAdapter(cfg, logger.WithModule("AidboxAdapter"))

	cacheAdapter, err := cache.NewCacheAdapter(cfg, logger.WithModule("CacheAdapter"))
	if err != nil {
		logger.Error("app.cache.init_failed", out.LogFields{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	// Инициализация сервиса
	slotGeneratorService := services.NewSlotGeneratorService(
		aidboxAdapter,
		cacheAdapter,
		logger.WithModule("SlotGeneratorService"),
	)

	// Настройка HTTP сервера
	router := gin.Default()
	controller := http.NewSlotGeneratorController(
		slotGeneratorService,
		cfg,
		logger.WithModule("HttpController"),
	)
	controller.RegisterRoutes(router)

	// Настройка RabbitMQ слушателя только если он включен
	if cfg.RabbitMq.Enabled {
		// Инициализация RabbitMQ слушателя
		rabbitListener, err := rabbitmq.NewCacheHitListener(cfg, slotGeneratorService, logger)
		if err != nil {
			logger.Error("rabbitmq.init_failed", out.LogFields{
				"error": err.Error(),
			})
		} else {
			// Запуск слушателя в отдельной горутине
			go func() {
				if err := rabbitListener.Start(context.Background()); err != nil {
					logger.Error("rabbitmq.start_failed", out.LogFields{
						"error": err.Error(),
					})
				}
			}()
		}
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("app.http.starting", out.LogFields{
			"host": cfg.HTTP.Host,
			"port": cfg.HTTP.Port,
		})

		if err := router.Run(cfg.HTTP.Host + ":" + cfg.HTTP.Port); err != nil {
			logger.Error("app.http.failed", out.LogFields{
				"error": err.Error(),
			})
			sigChan <- syscall.SIGTERM
		}
	}()

	sig := <-sigChan
	logger.Info("app.shutdown.initiated", out.LogFields{
		"signal": sig.String(),
	})

	// Дополнительное логирование для разработки
	if cfg.IsLocal() {
		logger.Debug("app.config.debug", out.LogFields{
			"config": map[string]interface{}{
				"http": map[string]string{
					"host": cfg.HTTP.Host,
					"port": cfg.HTTP.Port,
				},
				"aidbox": map[string]string{
					"url":      cfg.Aidbox.URL,
					"username": cfg.Aidbox.Username,
				},
				"rabbitmq": map[string]interface{}{
					"enabled": cfg.RabbitMq.Enabled,
					"url":     cfg.RabbitMq.AmqpUri,
					"queue":   cfg.RabbitMq.QueueConfig,
				},
				"cache": map[string]interface{}{
					"enabled":    cfg.Cache.Enabled,
					"slots_size": cfg.Cache.SlotsSize,
				},
			},
		})
	}
}
