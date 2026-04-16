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

  {
    const filePEM = "-----BEGIN RSA PRIVATE KEY-----\nfile-key-data\n-----END RSA PRIVATE KEY-----"
    const fakeReadFile = (path: string): string => {
      if (path === "/path/to/key.pem") return filePEM
      throw new Error(`Unexpected path: ${path}`)
    }

    assert({
      given: "QUARANTINE_APP_PRIVATE_KEY_PATH is set and QUARANTINE_APP_PRIVATE_KEY is not set",
      should: "return the PEM content read from the file",
      actual: resolvePrivateKey(undefined, "/path/to/key.pem", fakeReadFile),
      expected: filePEM,
    })
  }
})
