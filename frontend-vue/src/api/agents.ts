import { api, type ApiResponse } from './client'

// ==========================================
// Multi-Agent Framework Interfaces
// ==========================================

export interface ProjectState {
  id: string
  current_phase: string
  current_module: string
  current_feature: string
  current_task_id?: string
  current_subtask_id?: string
  repository_health: number
  technical_debt: number
  architecture_score: number
  quality_score: number
  updated_at: string
}

export type TaskStatus = 'pending' | 'inprogress' | 'success' | 'blocked' | 'failed'
export type SubtaskStatus = 'pending' | 'inprogress' | 'success' | 'failed'

export interface Subtask {
  id: string
  task_id: string
  title: string
  status: SubtaskStatus
  complexity: number
  exec_order: number
  completed_at?: string
}

export interface AgentTask {
  id: string
  phase: string
  module: string
  feature: string
  title: string
  description: string
  status: TaskStatus
  dependencies: string[]
  subtasks?: Subtask[]
  created_at: string
  updated_at: string
  completed_at?: string
}

export interface AgentExecution {
  id: string
  task_id: string
  agent_type: string
  status: 'running' | 'success' | 'failed'
  input?: string
  output?: string
  error_detail?: string
  created_at: string
  completed_at?: string
}

export interface CreateTaskPayload {
  phase: string
  module: string
  feature: string
  title: string
  description: string
  dependencies?: string[]
}

// ==========================================
// API Clients Export
// ==========================================

export const agentsApi = {
  async getState(): Promise<ProjectState> {
    return api.get<ProjectState>('/agents/state')
  },

  async listTasks(): Promise<AgentTask[]> {
    const res = await api.get<ApiResponse<AgentTask[]>>('/agents/tasks')
    return res.data || []
  },

  async createTask(payload: CreateTaskPayload): Promise<AgentTask> {
    return api.post<AgentTask>('/agents/tasks', payload)
  },

  async listRuns(taskId?: string): Promise<AgentExecution[]> {
    const params: Record<string, string> = {}
    if (taskId) params.task_id = taskId
    const res = await api.get<ApiResponse<AgentExecution[]>>('/agents/runs', params)
    return res.data || []
  },
}
