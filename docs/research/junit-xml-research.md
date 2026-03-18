# JUnit XML Format Research: classname/name Attributes Across Frameworks

## 1. Is There an Official JUnit XML Schema?

**No official schema exists from JUnit itself.** The closest thing is the Windy Road XSD (`JUnit.xsd`), which was created for Apache Ant's JUnit and JUnitReport tasks. The rspec_junit_formatter source explicitly references it: `http://windyroad.org/dl/Open%20Source/JUnit.xsd`. The schema is also mirrored on GitHub at `windyroad/JUnit-Schema`.

### What the Windy Road XSD Says About `<testcase>` Attributes

- **`name`** (`xs:token`, **required**): "Name of the test method"
- **`classname`** (`xs:token`, **required**): "Full class name for the class the test method is in."
- **`time`** (`xs:decimal`, **required**): execution time in seconds

The schema does **not** define a uniqueness constraint on `classname + name`. There is no `xs:unique` or `xs:key` element. The `xs:token` type allows any XML-safe characters with collapsed whitespace.

**In practice, no tool enforces uniqueness.** JUnit XML is a de facto standard with significant variation across producers.

---

## 2. Framework-by-Framework Analysis

### 2a. Jest via jest-junit

**Source:** `jest-community/jest-junit` on GitHub (master branch)

#### What `classname` Contains (Default)

Default template: `"{classname} {title}"`

Where:
- `{classname}` = `tc.ancestorTitles.join(ancestorSeparator)` -- the `describe()` block names joined together
- `{title}` = the `it()`/`test()` name
- Default `ancestorSeparator` = `" "` (single space)

So by default, **classname = all ancestor describe blocks + test title**, space-separated. This is identical to the default `name` attribute.

#### What `name` Contains (Default)

Default `titleTemplate`: `"{classname} {title}"` -- same as classname by default.

#### Configurability

Both are fully configurable via templates or functions:

| Config Key | Env Var | Default | Available Variables |
|---|---|---|---|
| `classNameTemplate` | `JEST_JUNIT_CLASSNAME` | `"{classname} {title}"` | `{classname}`, `{title}`, `{suitename}`, `{filepath}`, `{filename}`, `{displayName}` |
| `titleTemplate` | `JEST_JUNIT_TITLE` | `"{classname} {title}"` | `{classname}`, `{title}`, `{filepath}`, `{filename}`, `{displayName}` |
| `ancestorSeparator` | `JEST_JUNIT_ANCESTOR_SEPARATOR` | `" "` | N/A |

Can also be a function `(vars) => string` when configured in `jest.config.js`.

#### Uniqueness

- **Within a single run:** `classname + name` is unique **only if all test titles are unique within their describe hierarchy**. Jest itself enforces that `describe path + test name` is unique within a file, but with the default template (which duplicates classname into name), uniqueness depends on the full `ancestorTitles + title` being unique across all test files.
- **`test.each` edge case:** `test.each` generates distinct titles from the template string (e.g., `"adds %i + %i"` becomes `"adds 1 + 2"`, `"adds 3 + 4"`). These are unique **as long as the parameterized inputs produce distinct strings**. If two rows produce the same string, they will collide.
- **Cross-file:** The default template does NOT include the file path, so two files with identical describe/test structures will produce identical classname+name pairs.

#### Stability Across Runs

Yes -- `ancestorTitles` and `title` come from the source code and are deterministic. Stable as long as test code does not change.

#### Example XML (Default Config)

```xml
<!-- File: __tests__/addition.test.js -->
<!-- describe('addition', () => { describe('positive numbers', () => { it('should add up', ...) }) }) -->
<testsuites name="jest tests">
  <testsuite name="addition" tests="1" errors="0" failures="0" skipped="0"
             timestamp="2017-07-13T09:42:28" time="0.161">
    <testcase classname="addition positive numbers should add up"
              name="addition positive numbers should add up"
              time="0.004">
    </testcase>
  </testsuite>
</testsuites>
```

#### Example XML (Recommended Config: `classname="{classname}"`, `title="{title}"`)

```xml
<testcase classname="addition positive numbers"
          name="should add up"
          time="0.005">
</testcase>
```

---

### 2b. RSpec via rspec_junit_formatter

**Source:** `sj26/rspec_junit_formatter` on GitHub (main branch), files `lib/rspec_junit_formatter.rb` and `lib/rspec_junit_formatter/rspec3.rb`

#### What `classname` Contains

From `rspec3.rb`, the `classname_for` method:

```ruby
def classname_for(notification)
  fp = example_group_file_path_for(notification)
  fp.sub(%r{\.[^/]*\Z}, "").gsub("/", ".").gsub(%r{\A\.+|\.+\Z}, "")
end
```

This takes the **top-level example group's file path**, strips the file extension, replaces `/` with `.`, and trims leading/trailing dots.

Example: `./spec/models/user_spec.rb` becomes `spec.models.user_spec`

This is a **Java-style dotted classname** derived purely from the file path. It does NOT include describe block names.

#### What `name` Contains

From `rspec3.rb`, the `description_for` method:

```ruby
def description_for(notification)
  notification.example.full_description
end
```

This is RSpec's `full_description` -- the concatenation of all nested `describe`/`context` blocks plus the `it` description.

Example: `User#valid? returns true for valid attributes`

#### Configurability

**Not configurable.** There are no options, templates, or environment variables to change classname or name format. The formatter is intentionally simple.

#### Uniqueness

- **Within a single run:** The combination `classname + name` is unique as long as `full_description` values are unique within a file. RSpec does NOT enforce unique descriptions -- two `it` blocks with the same string in the same context will produce duplicate entries.
- **Shared examples (`shared_examples_for` / `it_behaves_like`):** These produce distinct `full_description` values because the host context is prepended. However, if two contexts include the same shared example and have the same nesting path, descriptions could collide.
- **`file_path` for classname:** Since classname is derived from the top-level `describe` group's file path (walking up to the root example group), it is the **spec file path**, not the shared example file path.
- **Parallel shards (parallel_tests):** The `TEST_ENV_NUMBER` env var is used in the `<testsuite name>` attribute (e.g., `rspec2`), but does NOT affect `classname` or `name`. Each shard runs different files, so classname+name should still be unique across shards as long as the same spec file is not split.

#### Stability Across Runs

Yes -- both `file_path` and `full_description` are derived from source code and are deterministic.

#### Example XML

```xml
<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="rspec" tests="3" skipped="0" failures="1" errors="0"
           time="0.034567" timestamp="2024-01-15T10:30:00+00:00"
           hostname="ci-runner-1">
  <properties>
    <property name="seed" value="12345"/>
  </properties>
  <testcase classname="spec.models.user_spec"
            name="User#valid? returns true for valid attributes"
            file="./spec/models/user_spec.rb"
            time="0.012345">
  </testcase>
  <testcase classname="spec.models.user_spec"
            name="User#valid? returns false for missing email"
            file="./spec/models/user_spec.rb"
            time="0.008901">
    <failure message="expected true, got false" type="RSpec::Expectations::ExpectationNotMetError">
      ...stack trace...
    </failure>
  </testcase>
  <testcase classname="spec.models.user_spec"
            name="User#email is pending implementation"
            file="./spec/models/user_spec.rb"
            time="0.000123">
    <skipped/>
  </testcase>
</testsuite>
```

Note: rspec_junit_formatter also emits a `file` attribute on `<testcase>`, which is the top-level example group's file path (e.g., `./spec/models/user_spec.rb`).

---

### 2c. Vitest (Built-in JUnit Reporter)

**Source:** `vitest-dev/vitest`, file `packages/vitest/src/node/reporters/junit.ts`

#### What `classname` Contains

Default: the **relative file path** from the project root (same as the `<testsuite name>`).

Example: `src/utils/__tests__/math.test.ts`

#### What `name` Contains

From the `flattenTasks` function, the name is built by joining all nested suite names with ` > ` separator, then appending the test name:

```typescript
function flattenTasks(task: Task, baseName = ''): Task[] {
  const base = baseName ? `${baseName} > ` : ''
  if (task.type === 'suite') {
    return task.tasks.flatMap(child => flattenTasks(child, `${base}${task.name}`))
  } else {
    return [{ ...task, name: `${base}${task.name}` }]
  }
}
```

Example: `math > add > should add positive numbers`

This is the full suite hierarchy + test name, joined with ` > `.

#### Configurability

The `classnameTemplate` option accepts a string with `{filename}` and `{filepath}` placeholders, or a function:

```typescript
classnameTemplate?: string | ((vars: { filename: string; filepath: string }) => string)
```

| Option | Default | Description |
|---|---|---|
| `classnameTemplate` | (file path) | Template for classname. Variables: `{filename}`, `{filepath}` |
| `suiteName` | `"vitest tests"` | Name for `<testsuites>` root |
| `addFileAttribute` | `false` | Add `file` attribute to `<testcase>` |

The `name` attribute is **not configurable**. It always uses the flattened suite path + test name.

#### Uniqueness

- **Within a single run:** `classname + name` is unique as long as no two tests in the same file have the same full suite path + test name. Vitest, like Jest, does not strictly enforce unique test names.
- **`test.each` / parameterized tests:** Same as Jest -- uniqueness depends on parameterized inputs producing distinct title strings.
- **Cross-file:** Since classname defaults to the file path, tests in different files always have different classnames.
- **Workspace projects:** `task.file.projectName` is available but is NOT included in classname or name by default.

#### Stability Across Runs

Yes -- file paths and test names are deterministic from source code.

#### Example XML

```xml
<?xml version="1.0" encoding="UTF-8" ?>
<testsuites name="vitest tests" tests="2" failures="0" errors="0" time="0.123">
  <testsuite name="src/utils/__tests__/math.test.ts"
             timestamp="2024-01-15T10:30:00.000Z"
             hostname="ci-runner-1"
             tests="2" failures="0" errors="0" skipped="0"
             time="0.045">
    <testcase classname="src/utils/__tests__/math.test.ts"
              name="math > add > should add positive numbers"
              time="0.012">
    </testcase>
    <testcase classname="src/utils/__tests__/math.test.ts"
              name="math > add > should handle negatives"
              time="0.008">
    </testcase>
  </testsuite>
</testsuites>
```

---

## 3. Characters in `classname` and `name`

### What Characters Can Appear?

All three frameworks XML-escape the standard problematic characters (`<`, `>`, `&`, `"`, `'`). Beyond that:

| Character | Jest | RSpec | Vitest | Problematic for composite key? |
|---|---|---|---|---|
| Spaces | Yes (in describe/test titles) | Yes (in descriptions) | Yes (in suite/test names) | No, if using a proper delimiter |
| `.` (dot) | Yes | Yes (classname uses dots as separator) | Yes (in file paths) | **Yes** -- collides with RSpec's path separator |
| `/` (slash) | Yes (in file paths if using `{filepath}`) | No (replaced with `.` in classname) | Yes (in file paths for classname) | **Yes** -- path separator |
| `:` (colon) | Rare but possible | Rare but possible | Rare but possible | Mildly problematic |
| `>` | Possible in test titles | Possible in descriptions | Used as separator (` > `) | **Yes** -- collides with Vitest hierarchy separator |
| `#` | Rare | Common in RSpec (e.g., `User#method`) | Rare | No |
| Unicode | Possible | Possible | Possible | No, if using UTF-8 throughout |
| Newlines | Stripped by jest-junit | Possible but unusual | Stripped | **Yes** -- breaks line-oriented processing |
| Backticks, parens, brackets | Common in `test.each` titles | Possible | Common in `test.each` | No |

### XML Escaping

All three frameworks escape `&`, `<`, `>`, `"`, and `'` in attribute values. RSpec additionally escapes illegal XML characters (control characters). Vitest removes invalid XML characters and discouraged characters.

---

## 4. Recommended `test_id` Construction Strategy

### Requirements

1. Unique within a single test run
2. Stable across runs (same test always produces the same ID)
3. Works across all three frameworks without configuration requirements
4. Resistant to collisions from edge cases (parameterized tests, shared examples)

### Strategy: `file_path::classname::name` with SHA-256 Fingerprint

**Raw composite key:** `{file_path}::{classname}::{name}`

**Hashed test_id:** `sha256(file_path + "::" + classname + "::" + name)` truncated to first 16 hex chars (64 bits)

### Why This Works

| Component | Jest | RSpec | Vitest |
|---|---|---|---|
| `file_path` | From `<testsuite>` or `file` attr if `addFileAttribute=true` | From `file` attr on `<testcase>` | From `<testsuite name>` (= relative file path) |
| `classname` | Configurable, defaults to ancestor chain + title | File-path-based dotted notation | File path (same as suite name by default) |
| `name` | Configurable, defaults to ancestor chain + title | `full_description` (all describe blocks + it text) | Flattened suite hierarchy ` > ` test name |

### Key Design Decisions

1. **Always include the file path as the first component.** This is the single most reliable disambiguator. Even with identical classname+name across frameworks, the file path ensures cross-file uniqueness. For Jest (where file path is not in classname by default), extract it from the `<testsuite>` context or require `addFileAttribute`.

2. **Use `::` as the delimiter** between components, not `.` or `/` or ` > `, since `::` does not naturally appear in any of the three frameworks' output.

3. **Hash the composite key** rather than storing the raw string. This normalizes length, avoids character-set issues in databases/APIs, and makes the ID safe for use as a JSON key, file name component, or URL segment.

4. **Store the raw components alongside the hash** for human readability and debugging. The `test_id` (hash) is for machine lookups; the `classname`, `name`, and `file` fields are for human display.

### Practical Construction (Pseudocode)

```go
func BuildTestID(testsuiteName, classname, testcaseName string) string {
    // Normalize: trim whitespace, normalize unicode (NFC)
    raw := testsuiteName + "::" + classname + "::" + testcaseName
    hash := sha256.Sum256([]byte(raw))
    return hex.EncodeToString(hash[:8]) // 16 hex chars = 64 bits
}
```

### Edge Cases and Mitigations

| Edge Case | Risk | Mitigation |
|---|---|---|
| `test.each` with duplicate param strings | Two tests get same name | SHA collision only if all three components match -- unlikely across different params. If needed, consumers can detect and append an index. |
| Shared examples in RSpec | Same description in different contexts | `full_description` includes the host context, so these differ. `file_path` also differs. |
| Jest default config (classname = name) | Redundant data, but not harmful | The file path (from testsuite) still disambiguates. Recommend users configure `classNameTemplate: "{classname}"` and `titleTemplate: "{title}"`. |
| Parallel shards writing separate XML files | Same test won't appear in multiple shards | Not a uniqueness problem; merge at the consuming side. |
| User reconfigures templates | Classname/name format changes between runs | test_id changes = test treated as new. Document that template config must be stable. |
| File renames | test_id changes | Expected behavior -- the test is effectively a new test at a new location. |

### Recommended `quarantine.yml` Guidance

To get the cleanest output, recommend (but do not require) these configurations:

**Jest:**
```json
{
  "classNameTemplate": "{classname}",
  "titleTemplate": "{title}",
  "ancestorSeparator": " > ",
  "addFileAttribute": "true"
}
```

**RSpec:** No configuration needed. Default output is already well-structured.

**Vitest:** No configuration needed. Default output includes file path in classname and hierarchical name.
