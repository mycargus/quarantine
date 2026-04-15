# Framework Detection

### Scenario 141: detect.Scan returns detected frameworks with source metadata [M11]

**Risk:** The detect package returns framework names without source information,
making it impossible for future UX to tell the user *where* a framework was
found (e.g., "jest found in package.json devDependencies").

**Given** a directory containing a `package.json` with `"jest": "^29.0.0"` in
`devDependencies` and a `Gemfile` containing `gem 'rspec'`

**When** `detect.Scan(dir)` is called

**Then** the result contains two `DetectedFramework` entries:
1. `{Name: "jest", Source: "package.json devDependencies"}`
2. `{Name: "rspec", Source: "Gemfile"}`

The function returns `Result` (never an error). Detection is advisory only.

---

### Scenario 142: detect.Scan prioritizes vitest over jest when both are present [M11]

**Risk:** A project migrating from jest to vitest has both in `package.json`.
If jest appears first, the generated config leads with the framework being
phased out, creating extra editing work for the user.

**Given** a directory containing a `package.json` with both `"jest": "^29.0.0"`
and `"vitest": "^1.0.0"` in `devDependencies`

**When** `detect.Scan(dir)` is called

**Then** the result contains two frameworks with vitest first:
1. `{Name: "vitest", Source: "package.json devDependencies"}`
2. `{Name: "jest", Source: "package.json devDependencies"}`

---

### Scenario 143: detect.Scan detects rspec-core in Gemfile [M11]

**Risk:** Some projects declare `gem 'rspec-core'` instead of `gem 'rspec'`.
If the detector only matches the exact string `rspec`, these projects get no
framework detection and must configure manually.

**Given** a directory containing a `Gemfile` with `gem "rspec-core", "~> 3.12"`

**When** `detect.Scan(dir)` is called

**Then** the result contains one framework: `{Name: "rspec", Source: "Gemfile"}`

---

### Scenario 144: detect.Scan ignores commented gems in Gemfile [M11]

**Risk:** A commented-out `gem 'rspec'` line causes a false positive detection,
pre-filling an rspec suite entry for a project that does not actually use rspec.

**Given** a directory containing a `Gemfile` with only `# gem 'rspec'`
(the gem line is commented out)

**When** `detect.Scan(dir)` is called

**Then** the result contains no frameworks. Commented lines are skipped.

---

### Scenario 145: detect.Scan silently returns empty result for malformed package.json [M11]

**Risk:** A malformed `package.json` causes `detect.Scan` to panic or return
an error, breaking `quarantine init` instead of silently falling back to
no-detection behavior.

**Given** a directory containing a `package.json` with invalid JSON content
(`{not valid json}`)

**When** `detect.Scan(dir)` is called

**Then** the result contains no frameworks. The malformed file is silently
ignored — `Scan` never returns an error.

---

### Scenario 146: quarantine doctor rejects per-suite retries outside 1-10 range [M11]

**Risk:** A user sets `retries: 20` on a suite entry. `quarantine doctor`
does not catch this, but `quarantine run` may behave unexpectedly or waste
CI time with excessive retries.

**Given** `.quarantine/config.yml` contains a valid suite with `retries: 20`

**When** the developer runs `quarantine doctor`

**Then** the CLI reports an error:
`suite "backend": invalid retries value: 20. Must be between 1 and 10.`

Exits with code 2.

---

### Scenario 147: quarantine doctor accepts per-suite retries of 0 as unset [M11]

**Risk:** A suite entry that omits `retries` (YAML default: 0) is rejected
by validation, forcing users to explicitly set retries on every suite even
when they want the default.

**Given** `.quarantine/config.yml` contains a valid suite with no `retries`
field (defaults to 0 in Go)

**When** the developer runs `quarantine doctor`

**Then** the CLI does NOT report any retries error for this suite. A retries
value of 0 means "use default" and is valid.
