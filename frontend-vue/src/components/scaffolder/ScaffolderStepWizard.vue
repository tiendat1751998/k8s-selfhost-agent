<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { Template } from '../../api/scaffold'
import {
  getFrameworkIcon,
  type RepoConfig,
  type CicdConfig,
} from '../../composables/useScaffolder'

defineProps<{
  show: boolean
  activeTemplate: Template | null
  currentStep: 1 | 2 | 3
  formVariables: Record<string, string>
  repoConfig: RepoConfig
  cicdConfig: CicdConfig
  registerInCatalog: boolean
  ownerTeam: string
  ownerEmail: string
  rendering: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:currentStep', step: 1 | 2 | 3): void
  (e: 'update:registerInCatalog', val: boolean): void
  (e: 'update:ownerTeam', val: string): void
  (e: 'update:ownerEmail', val: string): void
  (e: 'next-step'): void
  (e: 'prev-step'): void
  (e: 'submit', dryRun: boolean): void
}>()
</script>

<template>
  <ModalDrawer
    v-if="show && activeTemplate"
    :show="show"
    :title="`Scaffold: ${activeTemplate.name}`"
    @close="emit('close')"
  >
    <div class="wizard-modal-container">
      <!-- Template Summary Banner -->
      <div class="wizard-summary glass-panel">
        <div class="summary-avatar">{{ getFrameworkIcon(activeTemplate.framework) }}</div>
        <div class="summary-info">
          <h4>{{ activeTemplate.name }}</h4>
          <p>{{ activeTemplate.description }}</p>
          <div class="summary-meta">
            <span class="badge badge-cyan">{{ activeTemplate.framework }}</span>
            <span class="badge badge-indigo">{{ activeTemplate.category }}</span>
          </div>
        </div>
      </div>

      <!-- Stepper Navigation -->
      <div class="wizard-stepper">
        <div
          :class="['step-indicator', { active: currentStep === 1, completed: currentStep > 1 }]"
          @click="emit('update:currentStep', 1)"
        >
          <div class="step-circle">{{ currentStep > 1 ? '✅' : '1' }}</div>
          <span class="step-label">Parameters</span>
        </div>
        <div class="step-line"></div>
        <div
          :class="['step-indicator', { active: currentStep === 2, completed: currentStep > 2 }]"
          @click="emit('update:currentStep', 2)"
        >
          <div class="step-circle">{{ currentStep > 2 ? '✅' : '2' }}</div>
          <span class="step-label">Repo Config</span>
        </div>
        <div class="step-line"></div>
        <div
          :class="['step-indicator', { active: currentStep === 3 }]"
          @click="emit('update:currentStep', 3)"
        >
          <div class="step-circle">3</div>
          <span class="step-label">CI/CD & Catalog</span>
        </div>
      </div>

      <!-- Step 1: Parameters -->
      <div v-if="currentStep === 1" class="step-content-panel">
        <h4 class="section-title">⚙️ Step 1: Template Parameters</h4>
        <p class="section-sub">Configure dynamic parameters for Kubernetes and runtime templates.</p>

        <div class="variables-form-list">
          <div
            v-for="v in activeTemplate.variables"
            :key="v.name"
            class="form-group"
          >
            <label class="form-label">
              {{ v.label || v.name }}
              <span v-if="v.required" class="required-star">*</span>
            </label>

            <!-- Select Dropdown -->
            <select
              v-if="v.type === 'select'"
              v-model="formVariables[v.name]"
              class="form-select"
            >
              <option
                v-for="opt in (v.options || [])"
                :key="opt"
                :value="opt"
              >
                {{ opt }}
              </option>
            </select>

            <!-- Number / String Input -->
            <input
              v-else
              v-model="formVariables[v.name]"
              :type="v.type === 'number' ? 'number' : 'text'"
              :placeholder="v.default || ''"
              class="form-input"
            />
          </div>
        </div>
      </div>

      <!-- Step 2: Repository Config -->
      <div v-else-if="currentStep === 2" class="step-content-panel">
        <h4 class="section-title">📦 Step 2: Git Repository Configuration</h4>
        <p class="section-sub">Specify where the scaffolded source and infrastructure code will be created.</p>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Git Provider</label>
            <select v-model="repoConfig.gitProvider" class="form-select">
              <option value="github">GitHub</option>
              <option value="gitlab">GitLab</option>
              <option value="bitbucket">Bitbucket</option>
            </select>
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Organization / Group</label>
            <input
              v-model="repoConfig.organization"
              type="text"
              placeholder="e.g. engineering-org"
              class="form-input"
            />
          </div>
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Repository Name <span class="required-star">*</span></label>
            <input
              v-model="repoConfig.repoName"
              type="text"
              placeholder="my-microservice"
              class="form-input"
            />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Default Branch</label>
            <input
              v-model="repoConfig.branch"
              type="text"
              placeholder="main"
              class="form-input"
            />
          </div>
        </div>

        <div class="glass-panel p-3 rounded-lg mt-2">
          <label class="checkbox-label">
            <input v-model="repoConfig.isPrivate" type="checkbox" />
            <span class="checkbox-custom"></span>
            <span>Private Repository (Restricted to organization members)</span>
          </label>
        </div>
      </div>

      <!-- Step 3: CI/CD & Service Catalog -->
      <div v-else class="step-content-panel">
        <h4 class="section-title">🚀 Step 3: CI/CD Pipeline & Catalog Registry</h4>
        <p class="section-sub">Configure automatic deployment pipelines and catalog ownership metadata.</p>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Pipeline Engine</label>
            <select v-model="cicdConfig.pipelineProvider" class="form-select">
              <option value="github-actions">GitHub Actions (.github/workflows)</option>
              <option value="gitlab-ci">GitLab CI/CD (.gitlab-ci.yml)</option>
              <option value="argocd">ArgoCD GitOps Manifests</option>
            </select>
          </div>
        </div>

        <div class="glass-panel p-3 rounded-lg">
          <label class="checkbox-label">
            <input v-model="cicdConfig.triggerOnPush" type="checkbox" />
            <span class="checkbox-custom"></span>
            <span>Trigger automated build & test on branch push</span>
          </label>
        </div>

        <!-- Service Catalog Integration -->
        <div class="catalog-integration-box glass-panel">
          <label class="checkbox-label">
            <input
              :checked="registerInCatalog"
              type="checkbox"
              @change="emit('update:registerInCatalog', ($event.target as HTMLInputElement).checked)"
            />
            <span class="checkbox-custom"></span>
            <span>Register in Service Catalog automatically</span>
          </label>

          <transition name="fade">
            <div v-if="registerInCatalog" class="catalog-subfields">
              <div class="form-group">
                <label class="form-label">Owner Team</label>
                <input
                  :value="ownerTeam"
                  type="text"
                  placeholder="e.g. platform-team, backend"
                  class="form-input"
                  @input="emit('update:ownerTeam', ($event.target as HTMLInputElement).value)"
                />
              </div>
              <div class="form-group">
                <label class="form-label">Owner Email</label>
                <input
                  :value="ownerEmail"
                  type="email"
                  placeholder="devs@example.com"
                  class="form-input"
                  @input="emit('update:ownerEmail', ($event.target as HTMLInputElement).value)"
                />
              </div>
            </div>
          </transition>
        </div>
      </div>

      <!-- Stepper Actions Footer -->
      <div class="wizard-footer-actions">
        <div>
          <button
            v-if="currentStep > 1"
            class="btn-secondary"
            :disabled="rendering"
            @click="emit('prev-step')"
          >
            ◀️ Previous
          </button>
        </div>

        <div class="wizard-footer-right">
          <button
            v-if="currentStep < 3"
            class="btn-primary"
            @click="emit('next-step')"
          >
            Next Step ▶️
          </button>

          <template v-else>
            <button
              class="btn-secondary"
              :disabled="rendering"
              @click="emit('submit', true)"
            >
              <span v-if="rendering" class="btn-icon spin-anim">🔄</span>
              <span v-else class="btn-icon">🧪</span>
              Dry-Run Validation
            </button>

            <button
              class="btn-primary"
              :disabled="rendering"
              @click="emit('submit', false)"
            >
              <span v-if="rendering" class="btn-icon spin-anim">🔄</span>
              <span v-else class="btn-icon">🚀</span>
              {{ rendering ? 'Generating Manifests...' : 'Generate & Deploy' }}
            </button>
          </template>
        </div>
      </div>
    </div>
  </ModalDrawer>
</template>
