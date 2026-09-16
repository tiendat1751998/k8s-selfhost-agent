export interface StackTraceInfo {
  isStackTrace: boolean
  language?: 'java' | 'python' | 'go' | 'nodejs' | 'generic'
  firstLine: string
  linesCount: number
  stackLines: string[]
}

const PYTHON_REGEX = /(?:Traceback \(most recent call last\):|^\s*File\s+"[^"]+",\s+line\s+\d+)/m
const GO_REGEX = /(?:^goroutine\s+\d+\s+\[[^\]]+\]:|\t[^\n]+\.go:\d+(?:\s+\+0x[0-9a-fA-F]+)?)/m
const NODE_REGEX = /^\s*at\s+(?:.+?\s+)?\(?(?:node:|[a-zA-Z]:[\\/]|[/\\].+?|\b\w+?\.(?:js|ts|mjs|cjs|vue)):(?:\d+:\d+|\d+)\)?/m
const JAVA_REGEX = /(?:^\s*at\s+[\w$./]+\([^)]+\.java:\d+\)|^\s*at\s+(?:com|org|net|io|java|javax|android|gov|edu)\.[\w$./]+|Caused by:\s+[\w$.]+)/m

export function detectStackTrace(rawMessage: string): StackTraceInfo {
  if (!rawMessage || typeof rawMessage !== 'string') {
    return {
      isStackTrace: false,
      firstLine: '',
      linesCount: 0,
      stackLines: [],
    }
  }

  const normalized = rawMessage.replace(/\r\n/g, '\n')
  const lines = normalized.split('\n')
  const firstLine = lines[0] || ''

  if (lines.length <= 1) {
    return {
      isStackTrace: false,
      firstLine,
      linesCount: lines.length,
      stackLines: [],
    }
  }

  let language: StackTraceInfo['language'] = undefined
  const remainingText = lines.slice(1).join('\n')

  if (firstLine.includes('Traceback (most recent call last):') || PYTHON_REGEX.test(normalized)) {
    language = 'python'
  } else if (firstLine.startsWith('goroutine ') || GO_REGEX.test(remainingText)) {
    language = 'go'
  } else if (NODE_REGEX.test(remainingText)) {
    language = 'nodejs'
  } else if (JAVA_REGEX.test(remainingText)) {
    language = 'java'
  } else if (/^\s*at\s+/m.test(remainingText)) {
    language = 'generic'
  }

  if (!language) {
    return {
      isStackTrace: false,
      firstLine,
      linesCount: lines.length,
      stackLines: lines.slice(1),
    }
  }

  return {
    isStackTrace: true,
    language,
    firstLine,
    linesCount: lines.length,
    stackLines: lines.slice(1),
  }
}