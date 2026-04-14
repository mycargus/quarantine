# Dashboard Interface Tests

Interface tests that exercise the dashboard through its public HTTP interface
(`router.fetch()`), with external APIs (GitHub) stubbed. This is the MTP
Interface layer: tests that exercise a single component through its public
interface without crossing external service boundaries.

## Scope

These tests call `router.fetch(new Request(...))` via `createApp()`, exercising
route matching, parameter extraction, controller invocation, and response
rendering. External GitHub API calls are prevented by passing `token: ""` in
the `AppOptions` (an empty string is falsy, so the `if (token)` sync guard
never fires).

What belongs here:

- Tests that exercise routing and route parameter extraction
- Tests that verify correct HTTP status codes and response shapes
- Tests that require a real SQLite database (temp file, isolated per test)
- Tests that verify the full request → response path

What does NOT belong here:

- Pure function tests (put these next to the source file as `*.test.ts`)
- Tests that hit real external APIs (those are E2E tests in `test/e2e/`)

## File naming

```
<descriptive-name>.interface.test.ts
```

## Running

```bash
make dash-test   # runs unit and interface tests together
```
