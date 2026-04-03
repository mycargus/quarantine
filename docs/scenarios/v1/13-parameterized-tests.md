# Parameterized Tests

### Scenario 102: Jest test.each produces unique test_id per variant [M8]

**Risk:** Jest test.each variants collapse to a single test_id, making it impossible to quarantine one flaky variant without quarantining all others.

**Given** a Jest test file containing:
```javascript
test.each([
  [1, 2, 3],
  [4, 5, 9],
])('addition: %d + %d = %d', (a, b, expected) => { ... })
```
which produces JUnit XML entries:
- `<testcase classname="math" name="addition: 1 + 2 = 3" />`
- `<testcase classname="math" name="addition: 4 + 5 = 9" />`

**When** the CLI parses the JUnit XML and constructs test IDs

**Then** the two test cases receive distinct test_ids:
- `src/math.test.js::math::addition: 1 + 2 = 3`
- `src/math.test.js::math::addition: 4 + 5 = 9`

Each variant can be quarantined independently. Quarantining one does not suppress the others.

---

### Scenario 103: RSpec shared examples produce unique test_id per shared context [M8]

**Risk:** RSpec shared examples run under multiple contexts produce identical test_ids, causing quarantining one shared example to affect all contexts that include it.

**Given** an RSpec test file using shared examples:
```ruby
RSpec.shared_examples 'an admin user' do
  it 'has admin privileges' do ... end
end

describe 'User when admin' do
  include_examples 'an admin user'
end

describe 'ServiceAccount when admin' do
  include_examples 'an admin user'
end
```
which produces JUnit XML entries:
- `<testcase classname="User when admin" name="has admin privileges" file="./spec/models/user_spec.rb" />`
- `<testcase classname="ServiceAccount when admin" name="has admin privileges" file="./spec/models/user_spec.rb" />`

**When** the CLI parses the JUnit XML and constructs test IDs

**Then** the two test cases receive distinct test_ids because their `classname` attributes differ:
- `./spec/models/user_spec.rb::User when admin::has admin privileges`
- `./spec/models/user_spec.rb::ServiceAccount when admin::has admin privileges`

---

### Scenario 104: Vitest test.each produces unique test_id per variant [M8]

**Risk:** Vitest test.each variants receive the same test_id if the framework populates `classname` identically for all variants, preventing independent quarantine.

**Given** a Vitest test file at `src/processor.test.ts` containing:
```typescript
test.each([
  ['foo', 1],
  ['bar', 2],
])('processes %s with count %d', (input, count) => { ... })
```
which produces JUnit XML (from `<testsuite name="src/processor.test.ts">`) with entries:
- `<testcase classname="src/processor.test.ts" name="processes foo with count 1" />`
- `<testcase classname="src/processor.test.ts" name="processes bar with count 2" />`

**When** the CLI parses the JUnit XML and constructs test IDs

**Then** each variant receives a distinct test_id:
- `src/processor.test.ts::src/processor.test.ts::processes foo with count 1`
- `src/processor.test.ts::src/processor.test.ts::processes bar with count 2`

Each variant is tracked independently in `quarantine.json`.

---

### Scenario 105: Parameterized test name containing `::` does not corrupt the test_id [M8]

**Risk:** A test name containing `::` causes the test_id to be unparseable, breaking quarantine lookup and issue deduplication for any test with `::` in its name.

**Given** a Jest test with this JUnit XML entry:
```xml
<testcase classname="api" name="handles URL: https://api.example.com::v2" file="src/api.test.js" />
```

**When** the CLI constructs the test_id

**Then** the constructed test_id is:
`src/api.test.js::api::handles URL: https://api.example.com::v2`

The `::` within the `name` component is preserved as-is. The raw `name` field is stored alongside `test_id` in `quarantine.json`, so the original test name is always recoverable. Lookup uses an exact string match on the full composite `test_id`, not a split-and-parse operation.

---

### Scenario 106: Multiple parameterized variants are quarantined independently [M8]

**Risk:** Quarantining one flaky variant of a parameterized test suppresses all variants, over-excluding results and hiding genuine failures in other variants.

**Given** `quarantine.json` contains exactly one entry:
`src/math.test.js::math::addition: 1 + 2 = 3`

And a Jest test suite includes three `test.each` variants producing test_ids:
- `src/math.test.js::math::addition: 1 + 2 = 3`
- `src/math.test.js::math::addition: 4 + 5 = 9`
- `src/math.test.js::math::addition: 10 + 20 = 30`

**When** the CLI reads the quarantine state and augments the test command with exclusions

**Then** only the variant matching the quarantined test_id is excluded from execution. The other two variants run normally. The quarantine list remains at exactly one entry — quarantining does not expand to cover sibling variants.

---
