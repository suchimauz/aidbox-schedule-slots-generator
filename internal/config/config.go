package config

import (
	"github.com/caarlos0/env/v6"
	"strings"
)

type Environment string

const (
	EnvLocal      Environment = "local"
	EnvDev        Environment = "dev"
	EnvStage      Environment = "stage"
	EnvProduction Environment = "production"
)

type Config struct {
	App struct {
		Version  string      `env:"APP_VERSION" envDefault:"local"`
		Env      Environment `env:"APP_ENV" envDefault:"local"`
		Timezone string      `env:"APP_TIMEZONE" envDefault:"Europe/Moscow"`
	}

	HTTP struct {
		Port string `env:"HTTP_SERVER_PORT" envDefault:"8080"`
		Host string `env:"HTTP_SERVER_HOST" envDefault:"localhost"`
	}

	Aidbox struct {
		URL      string `env:"AIDBOX_URL"`
		Username string `env:"AIDBOX_USERNAME"`
		Password string `env:"AIDBOX_PASSWORD"`
	}

	RabbitMQ struct {
		Enabled bool   `env:"RABBITMQ_ENABLED"`
		URL     string `env:"RABBITMQ_URL"`
		Queue   string `env:"RABBITMQ_QUEUE"`
	}

	Cache struct {
		Enabled bool `env:"CACHE_ENABLED"`
		Size    int  `env:"CACHE_SIZE" envDefault:"1000"`
	}
}

func NewConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	// Приведение окружения к нижнему регистру для унификации
	cfg.App.Env = Environment(strings.ToLower(string(cfg.App.Env)))

	// Настройка значений по умолчанию в зависимости от окружения
	switch cfg.App.Env {
	case EnvLocal:
		// В локальном окружении по умолчанию отключаем RabbitMQ и кэш
		if !cfg.RabbitMQ.Enabled {
			cfg.RabbitMQ.Enabled = false
			cfg.Cache.Enabled = false
		}
	case EnvDev, EnvStage, EnvProduction:
		// В остальных окружениях по умолчанию включаем RabbitMQ и кэш,
		// если явно не указано иное
		if !cfg.RabbitMQ.Enabled {
			cfg.RabbitMQ.Enabled = true
			cfg.Cache.Enabled = true
		}
	}

	// Если RabbitMQ включен, автоматически включаем кэш
	if cfg.RabbitMQ.Enabled {
		cfg.Cache.Enabled = true
	}

	return cfg, nil
}

func (c *Config) IsLocal() bool {
	return c.App.Env == EnvLocal
}