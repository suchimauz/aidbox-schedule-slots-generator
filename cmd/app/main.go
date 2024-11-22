package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/adapters/in/http"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/adapters/in/rabbitmq"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/adapters/out/aidbox"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/adapters/out/cache"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/services"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/logger"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/out"
)

func main() {
	// Загрузка конфигурации
	cfg, err := config.NewConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Инициализация логгера с таймзоной
	logger, err := logger.NewConsoleLogger(cfg.App.Timezone)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	logger = logger.WithModule("Main")

	logger.Info("app.starting", out.LogFields{
		"version":         cfg.App.Version,
		"env":             cfg.App.Env,
		"timezone":        cfg.App.Timezone,
		"rabbitmqEnabled": cfg.RabbitMQ.Enabled,
		"cacheEnabled":    cfg.Cache.Enabled,
	})

	// Настройка Gin в зависимости от окружения
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Инициализация адаптеров
	aidboxAdapter := aidbox.NewAidboxAdapter(cfg, logger.WithModule("AidboxAdapter"))
	cacheAdapter, err := cache.NewLRUCacheAdapter(cfg, logger.WithModule("CacheAdapter"))
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

	// Настройка RabbitMQ слушателя
	listener, err := rabbitmq.NewAppointmentListener(
		slotGeneratorService,
		cfg,
		logger.WithModule("RabbitMQListener"),
	)
	if err != nil {
		logger.Error("app.rabbitmq.init_failed", out.LogFields{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := listener.Start(ctx); err != nil {
		logger.Error("app.rabbitmq.start_failed", out.LogFields{
			"error": err.Error(),
		})
		os.Exit(1)
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

	cancel()
	if err := listener.Stop(); err != nil {
		logger.Error("app.rabbitmq.stop_failed", out.LogFields{
			"error": err.Error(),
		})
	}

	logger.Info("app.shutdown.completed", nil)

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
					"enabled": cfg.RabbitMQ.Enabled,
					"url":     cfg.RabbitMQ.URL,
					"queue":   cfg.RabbitMQ.Queue,
				},
				"cache": map[string]interface{}{
					"enabled": cfg.Cache.Enabled,
					"size":    cfg.Cache.Size,
				},
			},
		})
	}
}