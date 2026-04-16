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

  {
    const envPEM = "-----BEGIN RSA PRIVATE KEY-----\nenv-key-data\n-----END RSA PRIVATE KEY-----"
    const pathPEM = "-----BEGIN RSA PRIVATE KEY-----\npath-key-data\n-----END RSA PRIVATE KEY-----"
    const pathReader = (path: string): string => {
      if (path === "/mounted/secret.pem") return pathPEM
      throw new Error(`Unexpected path: ${path}`)
    }

    assert({
      given: "both QUARANTINE_APP_PRIVATE_KEY and QUARANTINE_APP_PRIVATE_KEY_PATH are set",
      should: "use the file path value (file path takes precedence)",
      actual: resolvePrivateKey(envPEM, "/mounted/secret.pem", pathReader),
      expected: pathPEM,
    })
  }

  {
    const missingFileReader = (_path: string): string => {
      throw Object.assign(new Error("ENOENT: no such file or directory"), {
        code: "ENOENT",
      })
    }

    const thrownMessage = (fn: () => unknown): string | null => {
      try {
        fn()
        return null
      } catch (e) {
        return e instanceof Error ? e.message : String(e)
      }
    }

    assert({
      given: "QUARANTINE_APP_PRIVATE_KEY_PATH points to a nonexistent file",
      should: "throw an error identifying the missing file path",
      actual: thrownMessage(() =>
        resolvePrivateKey(undefined, "/nonexistent/key.pem", missingFileReader),
      ),
      expected: "Private key file not found: /nonexistent/key.pem",
    })
  }
})
