# Makefile for coda

APP_NAME := coda
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

AUTH_PKG := github.com/sebasusnik/coda/internal/auth
LDFLAGS := -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME} -X ${AUTH_PKG}.DefaultClientID=${CODA_CLIENT_ID} -X ${AUTH_PKG}.DefaultClientSecret=${CODA_CLIENT_SECRET}"

.PHONY: build clean install test cross-compile dev dev-tui help

## build: Build the application
build:
	go build ${LDFLAGS} -o ${APP_NAME} .

## install: Install the application globally
install:
	go install ${LDFLAGS} .

## clean: Remove build artifacts
clean:
	rm -f ${APP_NAME}
	rm -f ${APP_NAME}-*

## test: Run tests
test:
	go test -v ./...

## cross-compile: Build for multiple platforms
cross-compile:
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${APP_NAME}-linux-amd64 .
	# Linux ARM64 (Raspberry Pi)
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o ${APP_NAME}-linux-arm64 .
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o ${APP_NAME}-darwin-amd64 .
	# macOS ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o ${APP_NAME}-darwin-arm64 .
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o ${APP_NAME}-windows-amd64.exe .

## dev: Run the application with go run
dev:
	go run ${LDFLAGS} . $(ARGS)

## dev-tui: Build locally, set up device, and launch tui for local testing
dev-tui:
	go build ${LDFLAGS} -o ${APP_NAME} . && ./${APP_NAME} device setup && ./${APP_NAME} ui

## tidy: Tidy go modules
tidy:
	go mod tidy

## fmt: Format Go code
fmt:
	go fmt ./...

## vet: Run go vet
vet:
	go vet ./...

## lint: Run linters (requires golangci-lint)
lint:
	golangci-lint run

## release: Create a release build
release: clean tidy fmt vet cross-compile

## help: Show this help message
help:
	@echo "Available targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'
