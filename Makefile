.PHONY: dev cli-build cli-test cli-lint cli-mutate dash-build dash-test dash-lint dash-typecheck test-build e2e-test e2e-lint contract-test contract-lint test-lint schemas-validate lint-all test-all check install-hooks

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

contract-lint:
	cd test && pnpm run lint:contract

e2e-test:
	cd test && pnpm run test:e2e

e2e-lint:
	cd test && pnpm run lint:e2e

test-lint:
	cd test && pnpm run lint

# --- Schemas ---

schemas-validate:
	@echo "Validating golden fixtures against JSON schemas..."
	@echo "TODO: Wire up schema validation (Go: santhosh-tekuri/jsonschema, TS: ajv)"

# --- Dev Setup ---

dev: _check-prereqs install-hooks
	cd cli && go mod download
	cd dashboard && pnpm install
	cd test && pnpm install

_check-prereqs:
	@command -v go >/dev/null 2>&1 || { echo "Error: go is not installed. Run 'asdf install' in the repo root."; exit 1; }
	@command -v node >/dev/null 2>&1 || { echo "Error: node is not installed. Run 'asdf install' in the repo root."; exit 1; }
	@command -v pnpm >/dev/null 2>&1 || { echo "Error: pnpm is not installed. Run 'npm install -g pnpm'."; exit 1; }

install-hooks:
	git config core.hooksPath .githooks

# --- Aggregate ---

check: lint-all dash-typecheck

lint-all: cli-lint dash-lint test-lint

test-all: cli-test dash-test contract-test e2e-test schemas-validate
