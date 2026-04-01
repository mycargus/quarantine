.PHONY: dev cli-build cli-test cli-lint cli-mutate dash-test dash-lint dash-lint-ci dash-typecheck test-build e2e-test e2e-lint e2e-lint-ci contract-test contract-lint contract-lint-ci test-lint lint-all test-all check install-hooks release

# --- CLI (Go) ---

cli-build:
	go build -o bin/quarantine ./cli/cmd/quarantine

cli-test:
	go test ./cli/...

cli-lint:
	golangci-lint run ./cli/...

cli-mutate:
	claude --model sonnet "/test-mutation cli"

# --- Dashboard (TypeScript) ---

dash-test:
	cd dashboard && pnpm test

dash-lint:
	cd dashboard && pnpm run lint

dash-lint-ci:
	cd dashboard && pnpm run lint:ci

dash-typecheck:
	cd dashboard && pnpm run typecheck

# --- Test Infrastructure (contract + e2e) ---

test-build:
	cd test && pnpm install

contract-test:
	@./scripts/run-contract-tests.sh

contract-lint:
	cd test && pnpm run lint:contract

contract-lint-ci:
	cd test && pnpm run lint:ci:contract

e2e-test:
	cd test && pnpm run test:e2e

e2e-lint:
	cd test && pnpm run lint:e2e

e2e-lint-ci:
	cd test && pnpm run lint:ci:e2e

test-lint:
	cd test && pnpm run lint

# --- Dev Setup ---

dev: _check-prereqs install-hooks
	go mod download
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

test-all: cli-test dash-test contract-test e2e-test

# --- Release ---

release:
	@./scripts/release.sh $(VERSION)
