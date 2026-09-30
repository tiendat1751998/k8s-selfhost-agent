import BaseIcon from './components/ui/BaseIcon.vue'

declare module 'vue' {
  export interface GlobalComponents {
    BaseIcon: typeof BaseIcon
  }
}

export {}
