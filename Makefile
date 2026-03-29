.PHONY: cli-build cli-test cli-lint cli-mutate dash-build dash-test dash-lint dash-typecheck test-build e2e-test contract-test test-lint schemas-validate lint-all test-all

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

dash-typecheck:
	cd dashboard && pnpm run typecheck

# --- Test Infrastructure (contract + e2e) ---

test-build:
	cd test && pnpm install

contract-test:
	cd test && pnpm run test:contract

e2e-test:
	cd test && pnpm run test:e2e

test-lint:
	cd test && pnpm run lint

# --- Schemas ---

schemas-validate:
	@echo "Validating golden fixtures against JSON schemas..."
	@echo "TODO: Wire up schema validation (Go: santhosh-tekuri/jsonschema, TS: ajv)"

# --- Aggregate ---

lint-all: cli-lint dash-lint test-lint

test-all: cli-test dash-test contract-test e2e-test schemas-validate
