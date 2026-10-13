export interface NavItem {
  path: string
  name: string
  icon: string
  sub: string
  badge?: string
}

export interface NavGroup {
  key: string
  label: string
  icon: string
  items: NavItem[]
}

export { navGroups } from '../config/navigation'
