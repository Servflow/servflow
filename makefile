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
	@echo "==> Checking observability exits (secret scrubbing)..."
	@# Loggers/tracers must be derived from the request context so secret
	@# values resolved by {{ secret }} are scrubbed on the way out. Global
	@# loggers and raw tracers bypass the scrubbing layer.
	@bad=$$(grep -rn 'zap\.L()\|zap\.S()' --include='*.go' pkg internal 2>/dev/null | grep -v '_test.go'); \
	if [ -n "$$bad" ]; then echo "forbidden global logger (use logging.FromContext):"; echo "$$bad"; exit 1; fi
	@bad=$$(grep -rn 'otel\.Tracer(\|\.Tracer(".*").Start(' --include='*.go' pkg internal 2>/dev/null | grep -v '_test.go' | grep -v 'pkg/tracing/'); \
	if [ -n "$$bad" ]; then echo "forbidden raw tracer (use pkg/tracing constructors):"; echo "$$bad"; exit 1; fi
	@bad=$$(grep -rn 'logging\.FromContext(context\.Background())' --include='*.go' pkg internal 2>/dev/null | grep -v '_test.go'); \
	if [ -n "$$bad" ]; then echo "forbidden context-less logger (thread the real ctx):"; echo "$$bad"; exit 1; fi
	@bad=$$(grep -rn 'logging\.GetNewLogger(\|logging\.Build(' --include='*.go' pkg internal 2>/dev/null | grep -v '_test.go' | grep -v 'pkg/logging/' | grep -v 'pkg/engine/server/engine.go' | grep -v 'logging:root-ok'); \
	if [ -n "$$bad" ]; then echo "forbidden root logger outside bootstrap (use logging.FromContext, or mark genuine load-time sites with // logging:root-ok):"; echo "$$bad"; exit 1; fi
	@bad=$$(grep -rnE '^[[:space:]]*"log"$$' --include='*.go' pkg internal 2>/dev/null | grep -v '_test.go'); \
	if [ -n "$$bad" ]; then echo "forbidden stdlib log import (use pkg/logging):"; echo "$$bad"; exit 1; fi

docker-build:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(DOCKER_IMAGE) \
		--build-arg GIT_USERNAME=$(GIT_USERNAME) \
		--build-arg GIT_PASSWORD=$(GIT_PASSWORD) .

docker-push:
	docker push $(DOCKER_IMAGE)
