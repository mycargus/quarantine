.PHONY: cli-build cli-test cli-lint cli-mutate dash-build dash-test dash-lint dash-mutate e2e-build e2e-test e2e-lint schemas-validate lint-all mutate-all test-all

# --- CLI (Go) ---

cli-build:
	cd cli && go build -o ../bin/quarantine ./cmd/quarantine

cli-test:
	cd cli && go clean -testcache && go test ./...

cli-lint:
	cd cli && golangci-lint run

cli-mutate:
	@echo "Running mutation tests on each package (gremlins requires per-package runs)..."
	cd cli && GREMLINS=$$(which gremlins 2>/dev/null || echo "$$(asdf where golang 2>/dev/null)/bin/gremlins") && \
	for pkg in ./internal/config ./internal/git ./internal/github ./internal/parser ./internal/quarantine ./cmd/quarantine; do \
		echo "--- $$pkg ---"; \
		$$GREMLINS unleash -S lctkvsr --invert-assignments $$pkg; \
	done

# --- Dashboard (TypeScript) ---

dash-build:
	cd dashboard && pnpm run build

dash-test:
	cd dashboard && pnpm test

dash-lint:
	cd dashboard && pnpm run lint

dash-mutate:
	cd dashboard && pnpm exec stryker run

# --- End-to-End ---

e2e-build:
	cd e2e && pnpm install

e2e-test:
	cd e2e && pnpm test

e2e-lint:
	cd e2e && pnpm run lint

# --- Schemas ---

schemas-validate:
	@echo "Validating golden fixtures against JSON schemas..."
	@echo "TODO: Wire up schema validation (Go: santhosh-tekuri/jsonschema, TS: ajv)"

# --- Aggregate ---

lint-all: cli-lint dash-lint e2e-lint

mutate-all: cli-mutate dash-mutate

test-all: cli-test dash-test e2e-test schemas-validate
