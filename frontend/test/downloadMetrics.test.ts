import { describe, expect, it } from "vitest"
import { formatTotalEta, parseBytes, parseSpeed, parseEta, formatSpeed, formatEta } from "../src/lib/downloadMetrics"

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

  it("parses ETA strings", () => {
    expect(parseEta("00:34")).toBe(34)
    expect(parseEta("01:02:03")).toBe(3723)
    expect(parseEta("5")).toBe(5)
    expect(parseEta("")).toBe(0)
  })

  it("formats speed values", () => {
    expect(formatSpeed(2097152)).toBe("2.00MiB/s")
    expect(formatSpeed(512)).toBe("512.00B/s")
    expect(formatSpeed(1536)).toBe("1.50KiB/s")
    expect(formatSpeed(0)).toBe("0.00B/s")
  })

  it("formats ETA values", () => {
    expect(formatEta(34)).toBe("00:34")
    expect(formatEta(3723)).toBe("01:02:03")
    expect(formatEta(0)).toBe("00:00")
  })
})
