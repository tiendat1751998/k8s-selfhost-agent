export interface ServicePortInfo {
  name?: string
  port: number
  targetPort?: number | string
  protocol?: string
  nodePort?: number
}

export interface IngressRuleHttpPath {
  path?: string
  pathType?: string
  backend?: {
    service?: {
      name?: string
      port?: {
        name?: string
        number?: number
      }
    }
  }
}

export interface IngressRule {
  host?: string
  http?: {
    paths?: IngressRuleHttpPath[]
  }
}
