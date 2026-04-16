import { describe } from "riteway"
import { resolvePrivateKey } from "./private-key.server.js"

const fakePEM = "-----BEGIN RSA PRIVATE KEY-----\nfake-key-data\n-----END RSA PRIVATE KEY-----"

const unusedReadFile = (_path: string): string => {
  throw new Error("readFile should not be called")
}

describe("resolvePrivateKey()", async (assert) => {
  assert({
    given: "QUARANTINE_APP_PRIVATE_KEY is set and QUARANTINE_APP_PRIVATE_KEY_PATH is not set",
    should: "return the PEM value from the env var",
    actual: resolvePrivateKey(fakePEM, undefined, unusedReadFile),
    expected: fakePEM,
  })
})
