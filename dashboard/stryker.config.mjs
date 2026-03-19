/** @type {import('@stryker-mutator/core').PartialStrykerOptions} */
export default {
  // No native node:test runner exists; command runner checks exit code.
  testRunner: "command",
  commandRunner: {
    command: 'node --test --import tsx "app/**/*.test.{ts,tsx}"',
  },
  checkers: ["typescript"],
  tsconfigFile: "tsconfig.json",
  mutate: [
    "app/**/*.ts",
    "app/**/*.tsx",
    "!app/**/*.test.{ts,tsx}",
    "!app/root.tsx",
    "!app/routes/_index.tsx",
  ],
  reporters: ["clear-text", "progress"],
  coverageAnalysis: "off", // command runner doesn't support coverage analysis
};
