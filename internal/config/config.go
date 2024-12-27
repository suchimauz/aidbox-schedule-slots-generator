package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

const (
	appName = "aidbox-schedule-slot-generator"
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

type (
	Config struct {
		App struct {
			Version  string      `envconfig:"APP_VERSION" default:"local"`
			Env      Environment `envconfig:"APP_ENV" default:"local"`
			Timezone string      `envconfig:"APP_TIMEZONE" default:"Europe/Moscow"`
		}

		HTTP struct {
			Port string `envconfig:"HTTP_SERVER_PORT" default:"8080"`
			Host string `envconfig:"HTTP_SERVER_HOST" default:"localhost"`
		}

		Aidbox struct {
			URL      string `envconfig:"AIDBOX_URL"`
			Username string `envconfig:"AIDBOX_USERNAME"`
			Password string `envconfig:"AIDBOX_PASSWORD"`
		}

		Auth struct {
			BasicClientsString string `envconfig:"AUTH_BASIC_CLIENTS" default:"schedule_generator:schedule_generator"`
			BasicClients       []ConfigBasicClient
		}

		RabbitMq RabbitMqConfig

		Cache struct {
			Enabled          bool `envconfig:"CACHE_ENABLED"`
			SlotsSize        int  `envconfig:"CACHE_SLOTS_SIZE" default:"1000"`
			ScheduleRuleSize int  `envconfig:"CACHE_SCHEDULERULE_SIZE" default:"1000"`
		}
	}

	RabbitMqConfig struct {
		Enabled bool `envconfig:"RABBITMQ_ENABLED"`

		Host     string `envconfig:"RABBITMQ_HOST"`
		Port     string `envconfig:"RABBITMQ_PORT"`
		User     string `envconfig:"RABBITMQ_USERNAME"`
		Password string `envconfig:"RABBITMQ_PASSWORD"`
		Vhost    string `envconfig:"RABBITMQ_VHOST"`

		QueueConfig RabbitMqQueueConfig

		AmqpUri string
	}

	RabbitMqQueueConfig struct {
		AllQueueExchange string `envconfig:"RABBITMQ_QUEUE_ALL_EXCHANGE" default:"aidbox.slot-generator-svc.topic"`
		AllQueueName     string `envconfig:"RABBITMQ_QUEUE_ALL_NAME" default:"slot-generator-svc.cache._all_"`
		AllQueueBind     string `envconfig:"RABBITMQ_QUEUE_ALL_BIND" default:"*.slot-generator-svc._all_.#"`

		ScheduleRuleQueueExchange string `envconfig:"RABBITMQ_QUEUE_SCHEDULERULE_EXCHANGE" default:"aidbox.slot-generator-svc.topic"`
		ScheduleRuleQueueName     string `envconfig:"RABBITMQ_QUEUE_SCHEDULERULE_NAME" default:"slot-generator-svc.cache.schedulerule"`
		ScheduleRuleQueueBind     string `envconfig:"RABBITMQ_QUEUE_SCHEDULERULE_BIND" default:"*.slot-generator-svc.schedulerule.#"`

		AppointmentQueueExchange string `envconfig:"RABBITMQ_QUEUE_APPOINTMENT_EXCHANGE" default:"aidbox.slot-generator-svc.topic"`
		AppointmentQueueName     string `envconfig:"RABBITMQ_QUEUE_APPOINTMENT_NAME" default:"slot-generator-svc.cache.appointment"`
		AppointmentQueueBind     string `envconfig:"RABBITMQ_QUEUE_APPOINTMENT_BIND" default:"*.slot-generator-svc.appointment.#"`
	}
)

func NewConfig() (*Config, error) {
	var cfg Config

	godotenv.Load()

	if err := envconfig.Process(appName, &cfg); err != nil {
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

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) IsLocal() bool {
	return c.App.Env == EnvLocal
}

func (c *Config) IsNotLocal() bool {
	return c.App.Env == EnvDev || c.App.Env == EnvStage || c.App.Env == EnvProduction
}

func (cfg *Config) validate() error {
	// Если RabbitMQ включен, то проверяем его настройки
	if cfg.RabbitMq.Enabled {
		if err := cfg.RabbitMq.validateAndSetUri(); err != nil {
			return err
		}
	}

	// Если RabbitMQ не включен, то кэш тоже не включаем
	if cfg.Cache.Enabled && !cfg.RabbitMq.Enabled {
		return errors.New("RabbitMQ is required for cache")
	}

	return nil
}

// Private func for set all FileStorageConfig fields required
func (rmqcfg *RabbitMqConfig) validateAndSetUri() error {
	if rmqcfg.Host == "" {
		return errors.New("RABBITMQ_HOST is required")
	}
	if rmqcfg.Port == "" {
		return errors.New("RABBITMQ_PORT is required")
	}
	if rmqcfg.User == "" {
		return errors.New("RABBITMQ_USERNAME is required")
	}
	if rmqcfg.Password == "" {
		return errors.New("RABBITMQ_PASSWORD is required")
	}

	rmqcfg.AmqpUri = fmt.Sprintf("amqp://%s:%s@%s:%s/%s", rmqcfg.User, rmqcfg.Password, rmqcfg.Host, rmqcfg.Port, rmqcfg.Vhost)

	fmt.Println("rmqcfg.AmqpUri", rmqcfg.AmqpUri)

	if err := rmqcfg.QueueConfig.validate(); err != nil {
		return err
	}

	return nil
}

func (qcfg *RabbitMqQueueConfig) validate() error {
	if qcfg.ScheduleRuleQueueName == "" {
		return errors.New("RABBITMQ_QUEUE_SCHEDULERULE_NAME is required")
	}
	if qcfg.ScheduleRuleQueueBind == "" {
		return errors.New("RABBITMQ_QUEUE_SCHEDULERULE_BIND is required")
	}
	if qcfg.AllQueueName == "" {
		return errors.New("RABBITMQ_QUEUE_ALL_NAME is required")
	}
	if qcfg.AllQueueBind == "" {
		return errors.New("RABBITMQ_QUEUE_ALL_BIND is required")
	}
	if qcfg.AppointmentQueueName == "" {
		return errors.New("RABBITMQ_QUEUE_APPOINTMENT_NAME is required")
	}
	if qcfg.AppointmentQueueBind == "" {
		return errors.New("RABBITMQ_QUEUE_APPOINTMENT_BIND is required")
	}

	return nil
}
