.PHONY: help
.DEFAULT_GOAL := help

CURRENT_REVISION = $(shell git rev-parse --short HEAD)

help:
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

all: dist/lambda/httpserver.zip dist/lambda/dispatcher.zip ## Build all

tmp/shoes-ecs-task:
	mkdir -p ./tmp
	wget https://github.com/whywaita/shoes-ecs-task/releases/download/v0.0.2/shoes-ecs-task-linux-amd64 -O ./tmp/shoes-ecs-task

dist/lambda/httpserver: ## Build httpserver
	mkdir -p ./dist/lambda
	GOOS=linux GOARCH=amd64 go build -o ./dist/lambda/httpserver ./lambda/httpserver

dist/lambda/httpserver.zip: dist/lambda/httpserver tmp/shoes-ecs-task ## Build httpserver.zip
	make dist/lambda/httpserver
	zip -j ./dist/lambda/httpserver.zip ./dist/lambda/httpserver ./tmp/shoes-ecs-task

dist/lambda/dispatcher: ## Build dispatcher
	mkdir -p ./dist/lambda
	GOOS=linux GOARCH=amd64 go build -o ./dist/lambda/dispatcher ./lambda/dispatcher

dist/lambda/dispatcher.zip: dist/lambda/dispatcher tmp/shoes-ecs-task ## Build dispatcher.zip
	make dist/lambda/dispatcher
	zip -j ./dist/lambda/dispatcher.zip ./dist/lambda/dispatcher ./tmp/shoes-ecs-task

clean: ## Clean
	rm -rf ./dist ./tmp

test: ## Exec test
	go test -v ./...

gen-docs: ## Generate docs
	cat docs/overview.puml | docker run --rm -i -e PLANTUML_LIMIT_SIZE=8192 think/plantuml -tpng > docs/overview.png