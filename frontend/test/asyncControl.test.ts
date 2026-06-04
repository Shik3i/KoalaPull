import { describe, expect, it, vi } from "vitest"
import { createLatestSerializedWriter, startSerialPoll } from "../src/lib/asyncControl"

describe("createLatestSerializedWriter", () => {
  it("serializes writes and carries newer changes into the next snapshot", async () => {
    const releases: Array<() => void> = []
    const writes: Array<{ theme: string; maxConcurrency: number }> = []
    const writer = createLatestSerializedWriter(
      { theme: "dark", maxConcurrency: 3 },
      (value) => new Promise<void>((resolve) => {
        writes.push(value)
        releases.push(resolve)
      }),
      () => undefined,
    )

    const first = writer.update({ theme: "light" })
    const second = writer.update({ maxConcurrency: 8 })
    await Promise.resolve()
    expect(writes).toEqual([{ theme: "light", maxConcurrency: 3 }])

    releases.shift()?.()
    await first
    await Promise.resolve()
    expect(writes).toEqual([
      { theme: "light", maxConcurrency: 3 },
      { theme: "light", maxConcurrency: 8 },
    ])

    releases.shift()?.()
    await second
  })

  it("rolls the latest failed write back to the last persisted value", async () => {
    const rollbacks: Array<{ theme: string; maxConcurrency: number }> = []
    let calls = 0
    const writer = createLatestSerializedWriter(
      { theme: "dark", maxConcurrency: 3 },
      async () => {
        calls++
        if (calls === 2) throw new Error("disk full")
      },
      (value) => rollbacks.push(value),
    )

    await writer.update({ theme: "light" })
    await expect(writer.update({ maxConcurrency: 8 })).rejects.toThrow("disk full")
    expect(writer.desired()).toEqual({ theme: "light", maxConcurrency: 3 })
    expect(rollbacks).toEqual([{ theme: "light", maxConcurrency: 3 }])
  })

  it("does not let late initial settings overwrite a user mutation", async () => {
    const writer = createLatestSerializedWriter(
      { theme: "dark" },
      async () => undefined,
      () => undefined,
    )
    await writer.update({ theme: "light" })
    expect(writer.initialize({ theme: "dark" })).toBe(false)
    expect(writer.desired()).toEqual({ theme: "light" })
  })
})

describe("startSerialPoll", () => {
  it("never overlaps checks and stops scheduling after cleanup", async () => {
    vi.useFakeTimers()
    let calls = 0
    let release: (() => void) | undefined
    const stop = startSerialPoll(
      () => new Promise<void>((resolve) => {
        calls++
        release = resolve
      }),
      3000,
      () => undefined,
    )

    expect(calls).toBe(1)
    await vi.advanceTimersByTimeAsync(6000)
    expect(calls).toBe(1)

    release?.()
    await Promise.resolve()
    await vi.advanceTimersByTimeAsync(3000)
    expect(calls).toBe(2)

    stop()
    release?.()
    await vi.runAllTimersAsync()
    expect(calls).toBe(2)
    vi.useRealTimers()
  })
})
