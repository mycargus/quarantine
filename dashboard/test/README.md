# Dashboard Integration Tests

Integration tests that verify the interaction of multiple dashboard components (TypeScript modules, SQLite database, etc.) working together. External systems (GitHub, Jenkins, Jira, etc.) are mocked.

## Scope

These tests exercise full component flows within the dashboard boundary. They differ from unit tests (colocated in `dashboard/app/`) which test pure functions in isolation.

What belongs here:

- Tests that require a real SQLite database
- Tests that exercise multiple modules working together
- Tests that mock external HTTP APIs (GitHub, Jenkins, Jira)

What does NOT belong here:

- Pure function tests (put these next to the source file as `*.test.ts`)
- Tests that hit real external APIs (those are E2E tests in `test/e2e/`)

## File naming

```
<descriptive-name>.integration.test.ts
```

## Running

```bash
make dash-test   # runs both unit and integration tests
```
