import type { FileInfo } from './types'

let apiKey = ''
let onUnauthorized: (() => Promise<void>) | null = null

export function setApiKey(key: string) {
  apiKey = key
}

export function setUnauthorizedHandler(handler: () => Promise<void>) {
  onUnauthorized = handler
}

async function request(path: string, options: RequestInit = {}): Promise<Response> {
  const res = await fetch('/api' + path, {
    ...options,
    headers: {
      'X-API-Key': apiKey,
      'Content-Type': 'text/plain',
      ...options.headers,
    },
  })
  if (res.status === 401 && onUnauthorized) {
    await onUnauthorized()
    // caller should retry or bail; return the 401 response as-is
  }
  return res
}

async function extractError(res: Response): Promise<string> {
  try {
    const body = await res.clone().json()
    if (body.error) return body.error
  } catch {
    // ignore
  }
  return res.statusText || `HTTP ${res.status}`
}

// ── projects ──────────────────────────────────────────────────────────────────

export async function listProjects(): Promise<string[]> {
  const res = await request('/projects')
  if (!res.ok) throw new Error(await extractError(res))
  const data = await res.json()
  return data.projects ?? []
}

export async function deleteProject(project: string): Promise<void> {
  const res = await request(`/projects/${enc(project)}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(await extractError(res))
}

// ── files ─────────────────────────────────────────────────────────────────────

export async function listFiles(project: string): Promise<FileInfo[]> {
  const res = await request(`/projects/${enc(project)}/files`)
  if (!res.ok) throw new Error(await extractError(res))
  const data = await res.json()
  return data.files ?? []
}

export async function getFile(project: string, file: string): Promise<string> {
  const res = await request(`/projects/${enc(project)}/files/${enc(file)}`)
  if (!res.ok) throw new Error(await extractError(res))
  return res.text()
}

export async function putFile(project: string, file: string, content: string): Promise<void> {
  const res = await request(`/projects/${enc(project)}/files/${enc(file)}`, {
    method: 'PUT',
    body: content,
  })
  if (!res.ok) throw new Error(await extractError(res))
}

export async function deleteFile(project: string, file: string): Promise<void> {
  const res = await request(`/projects/${enc(project)}/files/${enc(file)}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(await extractError(res))
}

const enc = encodeURIComponent
