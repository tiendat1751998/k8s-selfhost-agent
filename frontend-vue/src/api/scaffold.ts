import { api } from './client'

export type TemplateCategory = 'all' | 'web' | 'api' | 'worker' | 'database' | 'fullstack'

export interface TemplateVariable {
  name: string
  label: string
  type: 'string' | 'number' | 'boolean' | 'select'
  default: string
  required: boolean
  options?: string[]
}

export interface Template {
  id: string
  name: string
  description: string
  category: string
  framework: string
  manifest_yaml: string
  helm_values: string
  docker_compose: string
  variables: TemplateVariable[]
  tags: string[]
  built_in: boolean
  tenant_id: string
  created_at: string
  updated_at: string
}

export interface RenderRequest {
  template_id?: string
  custom_template?: Partial<Template>
  variables: Record<string, string>
  register_catalog?: boolean
  owner_team?: string
  owner_email?: string
}

export interface RenderResponse {
  rendered_yaml: string
  rendered_compose: string
  rendered_helm: string
  catalog_entry_id?: string
}

export interface TemplateFilter {
  category?: string
  framework?: string
  search?: string
}

export const scaffoldApi = {
  list: (filter?: TemplateFilter): Promise<Template[]> =>
    api.get<Template[]>('/scaffolder/templates', filter as Record<string, string | number> | undefined),

  get: (id: string): Promise<Template> =>
    api.get<Template>(`/scaffolder/templates/${encodeURIComponent(id)}`),

  create: (template: Partial<Template>): Promise<Template> =>
    api.post<Template>('/scaffolder/templates', template),

  update: (id: string, template: Partial<Template>): Promise<Template> =>
    api.put<Template>(`/scaffolder/templates/${encodeURIComponent(id)}`, template),

  delete: (id: string): Promise<void> =>
    api.delete<void>(`/scaffolder/templates/${encodeURIComponent(id)}`),

  render: (req: RenderRequest): Promise<RenderResponse> =>
    api.post<RenderResponse>('/scaffolder/render', req),
}
