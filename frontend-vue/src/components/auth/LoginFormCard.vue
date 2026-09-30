<template>
  <form class="login-form" @submit.prevent="$emit('submit')">
    <div class="form-group">
      <label for="email" class="form-label">Email Address</label>
      <input
        id="email"
        :value="email"
        type="email"
        required
        autocomplete="email"
        placeholder="user@k8s.local"
        class="input-glass form-input"
        :disabled="loading"
        @input="$emit('update:email', ($event.target as HTMLInputElement).value)"
      />
    </div>

    <div class="form-group">
      <label for="password" class="form-label">Password</label>
      <input
        id="password"
        :value="password"
        type="password"
        required
        autocomplete="current-password"
        placeholder="••••••••••••"
        class="input-glass form-input"
        :disabled="loading"
        @input="$emit('update:password', ($event.target as HTMLInputElement).value)"
      />
    </div>

    <div class="form-row-remember">
      <label class="checkbox-label">
        <input
          type="checkbox"
          :checked="rememberMe"
          class="form-checkbox"
          :disabled="loading"
          @change="$emit('update:rememberMe', ($event.target as HTMLInputElement).checked)"
        />
        <span>Remember credentials</span>
      </label>
    </div>

    <button
      type="submit"
      class="btn btn-primary login-btn"
      :disabled="loading || !email || !password"
    >
      <span v-if="loading" class="spinner"></span>
      <span>{{ loading ? 'Authenticating...' : 'Sign In to Console' }}</span>
    </button>
  </form>
</template>

<script setup lang="ts">
defineProps<{
  email: string
  password: string
  rememberMe?: boolean
  loading?: boolean
}>()

defineEmits<{
  (e: 'update:email', value: string): void
  (e: 'update:password', value: string): void
  (e: 'update:rememberMe', value: boolean): void
  (e: 'submit'): void
}>()
</script>
