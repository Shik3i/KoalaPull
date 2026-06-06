import { describe, expect, it } from "vitest"
import { countDuplicateUrls } from "../src/lib/duplicateWarnings"

describe("duplicateWarnings", () => {
  it("counts duplicates across queue and history", () => {
    expect(countDuplicateUrls(
      " https://example.com/watch?v=42 ",
      [
        { url: "https://example.com/watch?v=42" },
        { url: "https://example.com/watch?v=99" },
        { url: "HTTPS://EXAMPLE.COM/WATCH?V=42" },
      ],
      [
        { url: "https://example.com/watch?v=42" },
        { url: "https://example.com/watch?v=55" },
      ],
    )).toEqual({
      queueCount: 2,
      historyCount: 1,
    })
  })

  it("returns zero counts for blank target urls", () => {
    expect(countDuplicateUrls("", [{ url: "https://example.com/watch?v=42" }], [{ url: "https://example.com/watch?v=42" }])).toEqual({
      queueCount: 0,
      historyCount: 0,
    })
  })
})
