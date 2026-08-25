// 1. Exported Types
export type Job = {
    id: number
    tenant_id: number
    queue_id: number
    url: string
    method: string
    state: string
    attempt_count: number
    max_attempts: number
    locked_by: string
    locked_at?: string
    schedule_id: number
    cancel_requested?: boolean
    run_at?: string
    next_run_at?: string
    created_at: string
    updated_at: string
  }
  
  type RequestOptions = {
    method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
    body?: any
    token?: string
  }
  
  // 2. Private Helper Function
  async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const { method = 'GET', body, token } = options
    
    const headers: Record<string, string> = {}
  
    // Automatically attach token if provided
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }
  
    // Automatically set Content-Type if there is a body payload
    let configBody: string | undefined = undefined
    if (body) {
      headers['Content-Type'] = 'application/json'
      configBody = JSON.stringify(body)
    }
  
    // Vite proxy handles the '/api' prefix and forwards to Go
    const response = await fetch(`/api${path}`, {
      method,
      headers,
      body: configBody,
    })
  
    // Error handling: UI can catch this and read the message
    if (!response.ok) {
      const errorText = await response.text().catch(() => 'Unknown error')
      throw new Error(`API Error (${response.status}): ${errorText}`)
    }
  
    return response.json() as Promise<T>
  }
  
  // 3. Public Exported Functions
  export async function getToken(tenantId: number): Promise<string> {
    const data = await request<{ token: string; expires_at: string }>('/auth/token', {
      method: 'POST',
      body: { tenant_id: tenantId },
    })
    return data.token
  }
  
  export async function listJobs(
    token: string, 
    queue_id: number, 
    state?: string, 
    limit?: number
  ): Promise<Job[]> {
    // Build query string dynamically if parameters are passed
    const params = new URLSearchParams()
    params.append('queue_id',  String(queue_id))
    if (state) params.append('state', state)
    if (limit) params.append('limit', limit.toString())
  
    return request<Job[]>(`/jobs?${params.toString()}`, { token })
  }
  
  export async function getJob(token: string, id: string): Promise<Job> {
    return request<Job>(`/jobs/${id}`, { token })
  }
  
  export async function replayJob(token: string, id: string): Promise<Job> {
    return request<Job>(`/jobs/${id}/replay`, {
      method: 'POST',
      token,
    })
  }
  