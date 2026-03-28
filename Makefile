.PHONY: cli-build cli-test cli-lint cli-mutate dash-build dash-test dash-lint e2e-build e2e-test e2e-lint schemas-validate lint-all test-all

# --- CLI (Go) ---

cli-build:
	cd cli && go build -o ../bin/quarantine ./cmd/quarantine

cli-test:
	cd cli && go test ./...

cli-lint:
	cd cli && golangci-lint run

cli-mutate:
	claude --model sonnet "/test-mutation cli"

# --- Dashboard (TypeScript) ---

dash-build:
	cd dashboard && pnpm run build

dash-test:
	cd dashboard && pnpm test

dash-lint:
	cd dashboard && pnpm run lint

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

test-all: cli-test dash-test e2e-test schemas-validate
