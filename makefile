STATICCHECK_VERSION := 2025.1.1

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
	@bad=$$(grep -rn 'otel\.Tracer(\|\.Tracer(".*").Start(' --include='*.go' pkg internal 2>/dev/null | grep -v '_test.go' ); \
	if [ -n "$$bad" ]; then echo "forbidden raw tracer (actions trace through the host):"; echo "$$bad"; exit 1; fi
	@bad=$$(grep -rn 'logging\.FromContext(context\.Background())' --include='*.go' pkg internal 2>/dev/null | grep -v '_test.go'); \
	if [ -n "$$bad" ]; then echo "forbidden context-less logger (thread the real ctx):"; echo "$$bad"; exit 1; fi
	@bad=$$(grep -rn 'logging\.GetNewLogger(\|logging\.Build(' --include='*.go' pkg internal 2>/dev/null | grep -v '_test.go' | grep -v 'pkg/logging/' | grep -v 'logging:root-ok'); \
	if [ -n "$$bad" ]; then echo "forbidden root logger outside bootstrap (use logging.FromContext, or mark genuine load-time sites with // logging:root-ok):"; echo "$$bad"; exit 1; fi
	@bad=$$(grep -rnE '^[[:space:]]*"log"$$' --include='*.go' pkg internal 2>/dev/null | grep -v '_test.go'); \
	if [ -n "$$bad" ]; then echo "forbidden stdlib log import (use pkg/logging):"; echo "$$bad"; exit 1; fi

hooks:
	git config core.hooksPath .githooks
	@echo "==> commit-msg and pre-push hooks active"

lint-commits:
	@sh scripts/commit-msg_test.sh
	@sh scripts/commit-msg.sh --range $${RANGE:-origin/main..HEAD}

.PHONY: hooks lint-commits
