<script setup lang="ts">
import type { AIProvider, TestPromptResult } from '../../api/management'

defineProps<{
  providers: AIProvider[]
  modelValueProvider: string
  systemPrompt: string
  userPrompt: string
  isRunning: boolean
  promptResult: TestPromptResult | null
  promptError: string | null
}>()

const emit = defineEmits<{
  (e: 'update:modelValueProvider', val: string): void
  (e: 'update:systemPrompt', val: string): void
  (e: 'update:userPrompt', val: string): void
  (e: 'runPrompt'): void
  (e: 'selectTemplate', t: string): void
  (e: 'copyOutput'): void
}>()
</script>

<template>
  <div class="console-layout glass-panel animate-fade-in">
    <div class="console-input-pane">
      <div class="console-pane-header">
        <h3>Diagnostic Prompt Engine</h3>
        <div class="provider-select-wrap">
          <label>Target LLM:</label>
          <select 
            :value="modelValueProvider" 
            class="input-glass select-llm"
            @change="emit('update:modelValueProvider', ($event.target as HTMLSelectElement).value)"
          >
            <option value="default">Default Provider (Local Ollama)</option>
            <option v-for="p in providers" :key="p.name" :value="p.name">
              {{ p.name }} ({{ p.model }} - {{ p.type }})
            </option>
          </select>
        </div>
      </div>

      <!-- Quick Templates -->
      <div class="template-chips">
        <span class="template-label">Quick Prompts:</span>
        <button class="tchip" @click="emit('selectTemplate', 'rca')">🔍 OOM RCA</button>
        <button class="tchip" @click="emit('selectTemplate', 'netpol')">🛡️ NetworkPolicy</button>
        <button class="tchip" @click="emit('selectTemplate', 'cve')">⚠️ Trivy CVE Fix</button>
        <button class="tchip" @click="emit('selectTemplate', 'hpa')">📈 KEDA Scaler</button>
      </div>

      <div class="console-form-group">
        <label>System Instructions:</label>
        <textarea 
          :value="systemPrompt" 
          rows="2" 
          class="input-glass console-textarea" 
          placeholder="System prompt context..."
          @input="emit('update:systemPrompt', ($event.target as HTMLTextAreaElement).value)"
        ></textarea>
      </div>

      <div class="console-form-group">
        <label>User Diagnostic Prompt:</label>
        <textarea 
          :value="userPrompt" 
          rows="4" 
          class="input-glass console-textarea" 
          placeholder="Enter cluster incident or question..."
          @input="emit('update:userPrompt', ($event.target as HTMLTextAreaElement).value)"
        ></textarea>
      </div>

      <div class="console-actions">
        <button 
          class="btn btn-primary" 
          :disabled="isRunning || !userPrompt.trim()" 
          @click="emit('runPrompt')"
        >
          <span>{{ isRunning ? 'Synthesizing Response...' : '🚀 Execute Completion' }}</span>
        </button>
      </div>
    </div>

    <!-- Output Pane -->
    <div class="console-output-pane">
      <div class="output-header">
        <div class="output-title">LLM Streaming Telemetry</div>
        <div v-if="promptResult" class="output-metrics font-mono">
          <span>Tokens: {{ promptResult.prompt_tokens }}+{{ promptResult.response_tokens }}</span>
          <span>Time: {{ promptResult.duration_ms }}ms</span>
          <button class="btn-copy" title="Copy Output" @click="emit('copyOutput')">Copy</button>
        </div>
      </div>

      <div class="output-terminal">
        <div v-if="isRunning" class="output-loading">
          <div class="spinner"></div>
          <span>Streaming token inference across cluster AI mesh...</span>
        </div>

        <div v-else-if="promptError" class="output-empty text-rose">
          <span class="empty-emoji">⚠️</span>
          <span>{{ promptError }}</span>
        </div>

        <div v-else-if="promptResult" class="output-content font-mono">
          <pre>{{ promptResult.content }}</pre>
        </div>

        <div v-else class="output-empty">
          <span class="empty-emoji">🤖</span>
          <span>Ready for diagnostic prompt execution. Click "Execute Completion" above.</span>
        </div>
      </div>
    </div>
  </div>
</template>
