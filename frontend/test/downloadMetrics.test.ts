import { describe, expect, it } from "vitest"
import { formatTotalEta, parseBytes, parseSpeed } from "../src/lib/downloadMetrics"

describe("downloadMetrics", () => {
  it("parses binary and decimal byte strings", () => {
    expect(parseBytes("1 KiB")).toBe(1024)
    expect(parseBytes("1.5 MiB")).toBe(1572864)
    expect(parseBytes("2 GB")).toBe(2147483648)
    expect(parseBytes("")).toBe(0)
  })

  it("parses speed strings", () => {
    expect(parseSpeed("2 MiB/s")).toBe(2097152)
    expect(parseSpeed("0.5 KiB/s")).toBe(512)
  })

  it("formats ETA text for short and long durations", () => {
    expect(formatTotalEta(0)).toBe("")
    expect(formatTotalEta(42)).toBe("<42s")
    expect(formatTotalEta(125)).toBe("~2:05")
    expect(formatTotalEta(3723)).toBe("~1:02:03")
  })
})
