# Версия приложения
VERSION := $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Переменные сборки
BINARY_NAME ?= schedule-slots-generator
BUILD_DIR ?= build

# Переменные окружения по умолчанию
APP_VERSION  ?= local
APP_ENV      ?= local
APP_TIMEZONE ?= Europe/Moscow

HTTP_SERVER_PORT ?= 8982
HTTP_SERVER_HOST ?= localhost

AIDBOX_URL     ?= http://localhost:8080
AIDBOX_USERNAME ?= slot-generator
AIDBOX_PASSWORD ?= local

RABBITMQ_ENABLED ?= false

CACHE_SIZE ?= 1000

ifneq ("$(wildcard .env)","")
	include .env
endif

.EXPORT_ALL_VARIABLES:
.PHONY: build run compose-build compose-build-force compose-run
.DEFAULT_GOAL := run

build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)" ./cmd/app

run: build
	@echo "Running $(BINARY_NAME)..."
	@./$(BUILD_DIR)/$(BINARY_NAME)

compose-build:
	docker-compose up --remove-orphans aidbox_schedule_slots_generator_builder

compose-build-force:
	docker-compose up --build aidbox_schedule_slots_generator_builder

compose-run: compose-build
	docker-compose up --remove-orphans aidbox_schedule_slots_generator_runner


# Вспомогательная команда для просмотра конфигурации
show-config:
	@echo "Environment configuration:"
	@echo "APP_VERSION:      $(APP_VERSION)"
	@echo "APP_ENV:          $(APP_ENV)"
	@echo "APP_TIMEZONE:     $(APP_TIMEZONE)"
	@echo "HTTP_SERVER_PORT: $(HTTP_SERVER_PORT)"
	@echo "HTTP_SERVER_HOST: $(HTTP_SERVER_HOST)"
	@echo "AIDBOX_URL:       $(AIDBOX_URL)"
	@echo "AIDBOX_USERNAME:  $(AIDBOX_USERNAME)"
	@echo "RABBITMQ_ENABLED: $(RABBITMQ_ENABLED)"
	@echo "RABBITMQ_URL:     $(RABBITMQ_URL)"
	@echo "RABBITMQ_QUEUE:   $(RABBITMQ_QUEUE)"
	@echo "CACHE_SIZE:       $(CACHE_SIZE)"
