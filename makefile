STATICCHECK_VERSION := 2025.1.1

HOST := servflow
IMAGE_NAME := servflow
TAG := $(shell git describe --tags --abbrev=0)
DOCKER_IMAGE := $(HOST)/$(IMAGE_NAME):$(TAG)

setup-lint-tools:
	go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)

lint:
	@echo "==> Checking gofmt..."
	@if [ -n "$$(gofmt -l .)" ]; then echo "Code not formatted properly"; gofmt -d .; exit 1; fi
	@echo "==> Running staticcheck..."
	staticcheck ./...

docker-build:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(DOCKER_IMAGE) \
		--build-arg GIT_USERNAME=$(GIT_USERNAME) \
		--build-arg GIT_PASSWORD=$(GIT_PASSWORD) .

docker-push:
	docker push $(DOCKER_IMAGE)
