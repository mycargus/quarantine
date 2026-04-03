import AdmZip from "adm-zip"

export function makeZipBuffer(jsonContent: string): Buffer {
  const zip = new AdmZip()
  zip.addFile("results.json", Buffer.from(jsonContent, "utf8"))
  return zip.toBuffer()
}

export function toArrayBuffer(buf: Buffer): ArrayBuffer {
  return buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength) as ArrayBuffer
}

export async function bodyText(response: Response): Promise<string> {
  return new Response(response.body).text()
}
