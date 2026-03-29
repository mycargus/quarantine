# Fixtures

Shared test fixtures for use across contract and E2E suites.

This directory is reserved for cross-suite fixtures — test data that both `contract/` and `e2e/` need. Suite-specific fixtures should live alongside the test files that use them.

Note: Go unit/integration test fixtures live in `testdata/` at the repository root (shared across Go packages). This directory is for the JavaScript test suites only.
