/**
 * Docker & Swarm Container & Image Formatting Utilities
 *
 * Provides intelligent parsing and human-readable formatting for:
 * 1. Docker Swarm task container names (e.g. `tiki_drone.1.fxj67kbqm3i13vcoh58nacp5h` -> `tiki_drone` + badge `slot .1 · fxj67kb...`)
 * 2. Standalone Docker container names (strips leading `/` e.g. `/postgres_db` -> `postgres_db`)
 * 3. Container image digests (shortens long sha256 references e.g. `drone/drone:2@sha256:55897c8fb...` -> `drone/drone:2 @55897c8...`)
 */

export interface FormattedContainerName {
  /** Original unformatted name */
  raw: string
  /** Cleaned full container name (leading slashes removed) */
  fullName: string
  /** Primary readable service / container name */
  serviceName: string
  /** Whether the container is a Docker Swarm task */
  isSwarmTask: boolean
  /** Swarm task slot number if applicable (e.g. '1') */
  slot?: string
  /** Full Swarm task ID hash if applicable */
  taskId?: string
  /** Shortened task ID hash (e.g. 'fxj67kb...') */
  shortHash?: string
  /** Formatted slot badge text (e.g. 'slot .1 · fxj67kb...') */
  slotBadgeText?: string
}

export interface FormattedImageName {
  /** Original unformatted image */
  raw: string
  /** Base image name with tag if present (e.g. 'drone/drone:2') */
  baseImage: string
  /** Whether image contains a digest/sha reference */
  hasDigest: boolean
  /** Digest algorithm (e.g. 'sha256') */
  algorithm?: string
  /** Full digest hash */
  digest?: string
  /** Shortened digest string (e.g. '@55897c8...') */
  shortDigest?: string
  /** Clean human-readable display string */
  display: string
}

/**
 * Parses and formats Docker / Swarm container names.
 *
 * Pattern: `<service_name>.<slot>.<task_id_hash>`
 * Example: `tiki_drone.1.fxj67kbqm3i13vcoh58nacp5h`
 * Returns: serviceName: `tiki_drone`, slotBadgeText: `slot .1 · fxj67kb...`
 */
export function formatContainerName(rawName?: string | null): FormattedContainerName {
  if (!rawName) {
    return {
      raw: '',
      fullName: '',
      serviceName: '',
      isSwarmTask: false,
    }
  }

  // 1. Clean any leading slashes
  const cleanName = rawName.replace(/^\/+/, '').trim()

  // 2. Check Swarm task container pattern: <service_name>.<slot>.<task_id_hash>
  const swarmMatch = cleanName.match(/^(.+?)\.(\d+)\.([a-zA-Z0-9_-]+)$/)

  if (swarmMatch) {
    const serviceName = swarmMatch[1]
    const slot = swarmMatch[2]
    const taskId = swarmMatch[3]
    const shortHash = taskId.length > 7 ? `${taskId.slice(0, 7)}...` : taskId
    const slotBadgeText = `slot .${slot} · ${shortHash}`

    return {
      raw: rawName,
      fullName: cleanName,
      serviceName,
      isSwarmTask: true,
      slot,
      taskId,
      shortHash,
      slotBadgeText,
    }
  }

  return {
    raw: rawName,
    fullName: cleanName,
    serviceName: cleanName,
    isSwarmTask: false,
  }
}

/**
 * Formats container images, shortening long sha256 digests into neat labels.
 *
 * Example: `drone/drone:2@sha256:55897c8fb4d232598379482739482379423`
 * Returns: baseImage: `drone/drone:2`, shortDigest: `@55897c8...`, display: `drone/drone:2 @55897c8...`
 */
export function formatImageName(rawImage?: string | null): FormattedImageName {
  if (!rawImage) {
    return {
      raw: '',
      baseImage: '',
      hasDigest: false,
      display: '',
    }
  }

  const cleanImage = rawImage.trim()

  // Match pattern: base@algo:digest or @sha256:digest
  const digestMatch = cleanImage.match(/^(.*?)@([a-zA-Z0-9_-]+):([a-fA-F0-9]+)$/)
  if (digestMatch) {
    const baseImage = digestMatch[1] || ''
    const algorithm = digestMatch[2]
    const digest = digestMatch[3]
    const shortHash = digest.length > 7 ? `${digest.slice(0, 7)}...` : digest
    const shortDigest = `@${shortHash}`
    const display = baseImage ? `${baseImage} ${shortDigest}` : `${algorithm}:${shortHash}`

    return {
      raw: rawImage,
      baseImage: baseImage || `${algorithm}:${shortHash}`,
      hasDigest: true,
      algorithm,
      digest,
      shortDigest,
      display,
    }
  }

  // Standalone sha256:hash
  const rawShaMatch = cleanImage.match(/^([a-zA-Z0-9_-]+):([a-fA-F0-9]{12,})$/)
  if (rawShaMatch && (rawShaMatch[1] === 'sha256' || rawShaMatch[1] === 'sha512')) {
    const algorithm = rawShaMatch[1]
    const digest = rawShaMatch[2]
    const shortHash = `${digest.slice(0, 7)}...`
    return {
      raw: rawImage,
      baseImage: `${algorithm}:${shortHash}`,
      hasDigest: true,
      algorithm,
      digest,
      shortDigest: `@${shortHash}`,
      display: `${algorithm}:${shortHash}`,
    }
  }

  return {
    raw: rawImage,
    baseImage: cleanImage,
    hasDigest: false,
    display: cleanImage,
  }
}
