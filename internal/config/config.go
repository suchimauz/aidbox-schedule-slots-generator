package config

import (
	"strings"

	"github.com/caarlos0/env/v6"
)

type Environment string

const (
	EnvLocal      Environment = "local"
	EnvDev        Environment = "dev"
	EnvStage      Environment = "stage"
	EnvProduction Environment = "production"
)

type ConfigBasicClient struct {
	Username string
	Password string
}

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

	Auth struct {
		BasicClientsString string `env:"AUTH_BASIC_CLIENTS" envDefault:"schedule_generator:schedule_generator"`
		BasicClients       []ConfigBasicClient
	}

	RabbitMQ struct {
		Enabled bool   `env:"RABBITMQ_ENABLED"`
		URL     string `env:"RABBITMQ_URL"`
		Queue   string `env:"RABBITMQ_QUEUE"`
	}

	Cache struct {
		Enabled   bool `env:"CACHE_ENABLED"`
		SlotsSize int  `env:"CACHE_SLOTS_SIZE" envDefault:"1000"`
	}
}

func NewConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	// Приведение окружения к нижнему регистру для унификации
	cfg.App.Env = Environment(strings.ToLower(string(cfg.App.Env)))

	// Разделение клиентов Aidbox
	if cfg.Auth.BasicClients == nil {
		cfg.Auth.BasicClients = []ConfigBasicClient{}
	}
	clientPairs := strings.Split(cfg.Auth.BasicClientsString, ",")
	for _, pair := range clientPairs {
		parts := strings.Split(pair, ":")
		if len(parts) == 2 {
			cfg.Auth.BasicClients = append(cfg.Auth.BasicClients, ConfigBasicClient{
				Username: parts[0],
				Password: parts[1],
			})
		}
	}

	// Если RabbitMQ не включен, то кэш тоже не включаем
	if !cfg.RabbitMQ.Enabled {
		cfg.Cache.Enabled = false
	}

	return cfg, nil
}

func (c *Config) IsLocal() bool {
	return c.App.Env == EnvLocal
}

func (c *Config) IsNotLocal() bool {
	return c.App.Env == EnvDev || c.App.Env == EnvStage || c.App.Env == EnvProduction
}
