#!/bin/bash

cd /root
echo "Building ${BINARY_NAME}..."
go mod download
go build -o ${BUILD_DIR}/${BINARY_NAME} -ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" ./cmd/app
