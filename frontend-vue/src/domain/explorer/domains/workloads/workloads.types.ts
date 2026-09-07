export interface ContainerPort {
  name?: string
  containerPort: number
  protocol?: string
  hostPort?: number
}

export interface ContainerSpec {
  name: string
  image?: string
  ports?: ContainerPort[]
  command?: string[]
  args?: string[]
  env?: { name: string; value?: string }[]
}

export interface ContainerStateWaiting {
  reason?: string
  message?: string
}

export interface ContainerStateRunning {
  startedAt?: string
}

export interface ContainerStateTerminated {
  exitCode?: number
  reason?: string
  message?: string
  startedAt?: string
  finishedAt?: string
}

export interface ContainerState {
  waiting?: ContainerStateWaiting
  running?: ContainerStateRunning
  terminated?: ContainerStateTerminated
}

export interface ContainerStatus {
  name: string
  image?: string
  imageID?: string
  ready?: boolean
  restartCount?: number
  started?: boolean
  state?: ContainerState
  lastState?: ContainerState
}

export interface MergedContainerInfo {
  name: string
  image: string
  ports: string
  state: string
  stateType: 'running' | 'waiting' | 'terminated' | 'unknown'
  ready: boolean
  restarts: number
}
