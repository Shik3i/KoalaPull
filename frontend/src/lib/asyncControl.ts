export interface LatestSerializedWriter<T extends object> {
  initialize(value: T): boolean
  update(patch: Partial<T>): Promise<void>
  desired(): T
}

export function createLatestSerializedWriter<T extends object>(
  initial: T,
  write: (value: T) => Promise<void>,
  rollback: (value: T, error: unknown) => void,
): LatestSerializedWriter<T> {
  let desired = initial
  let persisted = initial
  let revision = 0
  let chain = Promise.resolve()

  return {
    initialize(value: T) {
      if (revision !== 0) return false
      desired = value
      persisted = value
      return true
    },
    update(patch: Partial<T>) {
      revision++
      const snapshot = { ...desired, ...patch }
      desired = snapshot
      const result = chain.then(async () => {
        try {
          await write(snapshot)
          persisted = snapshot
        } catch (error) {
          if (desired === snapshot) {
            desired = persisted
            rollback(persisted, error)
          }
          throw error
        }
      })
      chain = result.catch(() => undefined)
      return result
    },
    desired() {
      return desired
    },
  }
}

export function startSerialPoll(
  task: () => Promise<void>,
  intervalMs: number,
  onError: (error: unknown) => void,
): () => void {
  let stopped = false
  let timer: ReturnType<typeof setTimeout> | undefined

  const run = async () => {
    try {
      await task()
    } catch (error) {
      onError(error)
    } finally {
      if (!stopped) timer = setTimeout(run, intervalMs)
    }
  }

  void run()
  return () => {
    stopped = true
    if (timer !== undefined) clearTimeout(timer)
  }
}
