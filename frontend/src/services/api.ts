let token = ''

export function setToken(t: string) {
  token = t
}

export function getToken(): string {
  return token
}

interface RequestOptions extends RequestInit {
  params?: Record<string, string>
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { params, ...fetchOptions } = options
  
  // Build query string
  const url = new URL(path, window.location.origin)
  if (params) {
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined) {
        url.searchParams.append(k, String(v))
      }
    })
  }

  // Setup headers
  const headers = new Headers(fetchOptions.headers || {})
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  if (fetchOptions.body && typeof fetchOptions.body === 'string') {
    headers.set('Content-Type', 'application/json')
  }

  fetchOptions.headers = headers

  const res = await fetch(url.toString(), fetchOptions)
  
  if (!res.ok) {
    const errorText = await res.text().catch(() => '')
    throw new Error(`API Error ${res.status}: ${errorText}`)
  }

  const text = await res.text()
  if (!text) return {} as T
  return JSON.parse(text) as T
}

export const api = {
  get<T>(path: string, params?: Record<string, string>): Promise<T> {
    return request<T>('/api/v1' + path, { method: 'GET', params })
  },

  post<T>(path: string, body?: unknown): Promise<T> {
    return request<T>('/api/v1' + path, {
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
    })
  },

  put<T>(path: string, body?: unknown): Promise<T> {
    return request<T>('/api/v1' + path, {
      method: 'PUT',
      body: body ? JSON.stringify(body) : undefined,
    })
  },

  delete<T>(path: string, params?: Record<string, string>): Promise<T> {
    return request<T>('/api/v1' + path, { method: 'DELETE', params })
  },
}
