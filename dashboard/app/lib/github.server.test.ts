import AdmZip from "adm-zip"
import { describe } from "riteway/esm"
import { downloadAndExtractJson, listArtifacts } from "./github.server.js"

type FetchFn = typeof fetch

const makeArtifact = (id: number, name: string) => ({
  id,
  name,
  archive_download_url: `https://api.github.com/repos/mycargus/my-app/actions/artifacts/${id}/zip`,
  created_at: "2026-03-15T14:00:00Z",
  expires_at: "2026-04-15T14:00:00Z",
})

const makeZipBuffer = (content: string): ArrayBuffer => {
  const zip = new AdmZip()
  zip.addFile("results.json", Buffer.from(content, "utf8"))
  const buf = zip.toBuffer()
  return buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength) as ArrayBuffer
}

const throws = async (fn: () => Promise<unknown>): Promise<string | null> => {
  try {
    await fn()
    return null
  } catch (e) {
    return e instanceof Error ? e.message : String(e)
  }
}

describe("listArtifacts()", async (assert) => {
  assert({
    given: "a 200 response with artifacts and an ETag header",
    should: "return the artifact list and the etag",
    actual: await listArtifacts("mycargus", "my-app", "token-abc", null, (async (_url, _init) => ({
      ok: true,
      status: 200,
      headers: { get: (k: string) => (k === "etag" ? '"abc123"' : null) },
      json: async () => ({ artifacts: [makeArtifact(1, "quarantine-results-run-123")] }),
    })) as unknown as FetchFn),
    expected: {
      artifacts: [makeArtifact(1, "quarantine-results-run-123")],
      etag: '"abc123"',
      notModified: false,
    },
  })

  assert({
    given: "a 304 Not Modified response",
    should: "return notModified: true with empty artifacts",
    actual: await listArtifacts("mycargus", "my-app", "token-abc", '"abc123"', (async (
      _url,
      _init,
    ) => ({
      ok: false,
      status: 304,
      headers: { get: (_k: string) => null },
      json: async () => ({}),
    })) as unknown as FetchFn),
    expected: {
      artifacts: [],
      etag: null,
      notModified: true,
    },
  })

  assert({
    given: "a prior etag value",
    should: "send If-None-Match header in the request",
    actual: await (async () => {
      let capturedHeaders: Record<string, string> = {}
      await listArtifacts("mycargus", "my-app", "token-abc", '"etag-value"', (async (
        _url: string | URL | Request,
        init?: RequestInit,
      ) => {
        capturedHeaders = (init?.headers ?? {}) as Record<string, string>
        return {
          ok: false,
          status: 304,
          headers: { get: (_k: string) => null },
          json: async () => ({}),
        }
      }) as unknown as FetchFn)
      return capturedHeaders["If-None-Match"]
    })(),
    expected: '"etag-value"',
  })

  assert({
    given: "a null etag",
    should: "not send an If-None-Match header",
    actual: await (async () => {
      let capturedHeaders: Record<string, string> = {}
      await listArtifacts("mycargus", "my-app", "token-abc", null, (async (
        _url: string | URL | Request,
        init?: RequestInit,
      ) => {
        capturedHeaders = (init?.headers ?? {}) as Record<string, string>
        return {
          ok: true,
          status: 200,
          headers: { get: (_k: string) => null },
          json: async () => ({ artifacts: [] }),
        }
      }) as unknown as FetchFn)
      return "If-None-Match" in capturedHeaders
    })(),
    expected: false,
  })

  assert({
    given: "a 401 Unauthorized response",
    should: "throw an error with the status code",
    actual: (
      await throws(() =>
        listArtifacts("mycargus", "my-app", "bad-token", null, (async () => ({
          ok: false,
          status: 401,
          headers: { get: (_k: string) => null },
          json: async () => ({}),
        })) as unknown as FetchFn),
      )
    )?.includes("401"),
    expected: true,
  })

  assert({
    given: "a 500 Internal Server Error response",
    should: "throw an error with the status code",
    actual: (
      await throws(() =>
        listArtifacts("mycargus", "my-app", "token-abc", null, (async () => ({
          ok: false,
          status: 500,
          headers: { get: (_k: string) => null },
          json: async () => ({}),
        })) as unknown as FetchFn),
      )
    )?.includes("500"),
    expected: true,
  })
})

describe("downloadAndExtractJson()", async (assert) => {
  assert({
    given: "a valid zip containing a JSON file",
    should: "return the JSON string of the first entry",
    actual: await downloadAndExtractJson(
      "https://example.com/artifact.zip",
      "token-abc",
      (async () => ({
        arrayBuffer: async () => makeZipBuffer('{"run_id":"test-123"}'),
      })) as unknown as FetchFn,
    ),
    expected: '{"run_id":"test-123"}',
  })

  assert({
    given: "a zip with multiple entries",
    should: "return only the first entry's content",
    actual: await downloadAndExtractJson(
      "https://example.com/artifact.zip",
      "token-abc",
      (async () => {
        const zip = new AdmZip()
        zip.addFile("first.json", Buffer.from("first", "utf8"))
        zip.addFile("second.json", Buffer.from("second", "utf8"))
        const buf = zip.toBuffer()
        return {
          arrayBuffer: async () =>
            buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength) as ArrayBuffer,
        }
      }) as unknown as FetchFn,
    ),
    expected: "first",
  })

  assert({
    given: "a zip with no entries",
    should: "throw an error",
    actual:
      (await throws(() =>
        downloadAndExtractJson("https://example.com/empty.zip", "token-abc", (async () => {
          const zip = new AdmZip()
          const buf = zip.toBuffer()
          return {
            arrayBuffer: async () =>
              buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength) as ArrayBuffer,
          }
        }) as unknown as FetchFn),
      )) !== null,
    expected: true,
  })
})
