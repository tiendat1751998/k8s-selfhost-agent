import type { LogEntry } from '../stores/logStore'

export class LogCircularBuffer {
  private buffer: (LogEntry | undefined)[]
  private capacity: number
  private head: number = 0
  private tail: number = 0
  private count: number = 0

  constructor(capacity: number = 100000) {
    this.capacity = Math.max(10, capacity)
    this.buffer = new Array(this.capacity)
  }

  get length(): number {
    return this.count
  }

  get maxCapacity(): number {
    return this.capacity
  }

  push(entry: LogEntry): void {
    this.buffer[this.tail] = entry
    this.tail = (this.tail + 1) % this.capacity

    if (this.count < this.capacity) {
      this.count++
    } else {
      this.head = (this.head + 1) % this.capacity
    }
  }

  pushBatch(entries: LogEntry[]): void {
    if (!entries || entries.length === 0) return

    if (entries.length >= this.capacity) {
      // If batch itself is larger than capacity, keep only the latest items
      const slice = entries.slice(-this.capacity)
      this.buffer = new Array(this.capacity)
      for (let i = 0; i < slice.length; i++) {
        this.buffer[i] = slice[i]
      }
      this.head = 0
      this.tail = slice.length % this.capacity
      this.count = slice.length
      return
    }

    for (let i = 0; i < entries.length; i++) {
      this.push(entries[i])
    }
  }

  get(index: number): LogEntry | undefined {
    if (index < 0 || index >= this.count) return undefined
    const actualIndex = (this.head + index) % this.capacity
    return this.buffer[actualIndex]
  }

  toArray(): LogEntry[] {
    if (this.count === 0) return []
    const result = new Array<LogEntry>(this.count)
    for (let i = 0; i < this.count; i++) {
      result[i] = this.buffer[(this.head + i) % this.capacity] as LogEntry
    }
    return result
  }

  clear(): void {
    this.buffer = new Array(this.capacity)
    this.head = 0
    this.tail = 0
    this.count = 0
  }

  setCapacity(newCapacity: number): void {
    const validCap = Math.max(10, newCapacity)
    if (validCap === this.capacity) return

    const currentEntries = this.toArray()
    this.capacity = validCap
    this.buffer = new Array(validCap)
    this.head = 0
    this.tail = 0
    this.count = 0

    const toKeep = currentEntries.slice(-validCap)
    this.pushBatch(toKeep)
  }
}