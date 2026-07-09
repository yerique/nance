import type { H3Event } from 'h3'

/**
 * Resolve accelerator control-plane config at request time.
 *
 * Nuxt only auto-overrides runtimeConfig from NUXT_* env vars
 * (e.g. NUXT_ACCELERATOR_BASE_URL). We intentionally also honor the
 * monorepo NANCE_* names at runtime so Docker/prod can set
 * NANCE_ACCELERATOR_URL without rebuilding — process.env.NANCE_* in
 * nuxt.config is evaluated only at build/config load and is baked in.
 */
export function getAcceleratorConfig(event: H3Event) {
  const config = useRuntimeConfig(event)
  const baseUrl = ((process.env.NANCE_ACCELERATOR_URL || config.acceleratorBaseUrl) as string).replace(/\/$/, '')
  const adminToken = (process.env.NANCE_ADMIN_TOKEN || config.acceleratorAdminToken as string)
  return { baseUrl, adminToken }
}

export async function acceleratorFetch<T>(
  event: H3Event,
  path: string,
  options: {
    method?: string
    body?: unknown
    query?: Record<string, string>
    /** Prefer user session from Authorization header over server admin token */
    userAuth?: boolean
  } = {},
): Promise<T> {
  const { baseUrl, adminToken } = getAcceleratorConfig(event)
  const method = options.method || 'GET'

  const headers: Record<string, string> = {
    Accept: 'application/json',
  }

  const incoming = getHeader(event, 'authorization')
  if (options.userAuth !== false && incoming?.startsWith('Bearer ')) {
    headers.Authorization = incoming
  }
  else if (adminToken) {
    headers.Authorization = `Bearer ${adminToken}`
  }

  if (options.body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }

  let url = `${baseUrl}${path.startsWith('/') ? path : `/${path}`}`
  if (options.query) {
    const qs = new URLSearchParams(options.query).toString()
    if (qs) url += `?${qs}`
  }

  try {
    return await $fetch<T>(url, {
      method: method as 'GET',
      headers,
      body: options.body !== undefined ? options.body : undefined,
    })
  }
  catch (err: unknown) {
    const { statusCode, message } = extractUpstreamError(err)
    // Put the backend message on statusMessage and data.error (string).
    // Clients must not read the h3 boolean `error: true` as the message.
    throw createError({
      statusCode,
      statusMessage: message,
      message,
      data: { error: message },
    })
  }
}

/** Pull status + human message from ofetch errors against the Go control plane. */
function extractUpstreamError(err: unknown): { statusCode: number, message: string } {
  const e = err as {
    statusCode?: number
    status?: number
    statusMessage?: string
    message?: string
    data?: unknown
    response?: { status?: number, _data?: unknown }
  }

  const statusCode = e.statusCode || e.status || e.response?.status || 502
  const body = e.data ?? e.response?._data

  let message = 'Accelerator request failed'
  if (typeof body === 'string' && body.trim()) {
    message = body.trim()
  }
  else if (body && typeof body === 'object') {
    const b = body as Record<string, unknown>
    if (typeof b.error === 'string' && b.error.trim()) {
      message = b.error.trim()
    }
    else if (typeof b.message === 'string' && b.message.trim()) {
      message = b.message.trim()
    }
    else if (typeof b.statusMessage === 'string' && b.statusMessage.trim()) {
      message = b.statusMessage.trim()
    }
  }
  else if (typeof e.statusMessage === 'string' && e.statusMessage.trim()) {
    message = e.statusMessage.trim()
  }
  else if (typeof e.message === 'string' && e.message.trim() && !/^\[[A-Z]+\]\s+"/.test(e.message)) {
    message = e.message.trim()
  }

  return { statusCode, message }
}
