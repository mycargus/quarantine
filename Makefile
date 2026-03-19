.PHONY: cli-build cli-test cli-lint cli-mutate dash-build dash-test dash-lint dash-mutate schemas-validate test-all mutate-all

# --- CLI (Go) ---

cli-build:
	cd cli && go build -o ../bin/quarantine ./cmd/quarantine

cli-test:
	cd cli && go clean -testcache && go test ./...

cli-lint:
	cd cli && golangci-lint run

cli-mutate:
	@echo "Running mutation tests on each package (gremlins requires per-package runs)..."
	cd cli && for pkg in ./internal/config ./internal/git ./internal/github ./internal/parser ./internal/quarantine ./cmd/quarantine; do \
		echo "--- $$pkg ---"; \
		gremlins unleash -S lctkvsr --invert-assignments $$pkg; \
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

# --- Schemas ---

schemas-validate:
	@echo "Validating golden fixtures against JSON schemas..."
	@echo "TODO: Wire up schema validation (Go: santhosh-tekuri/jsonschema, TS: ajv)"

# --- Aggregate ---

test-all: cli-test dash-test schemas-validate

mutate-all: cli-mutate dash-mutate
