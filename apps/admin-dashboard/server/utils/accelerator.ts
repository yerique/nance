import type { H3Event } from 'h3'

export function getAcceleratorConfig(event: H3Event) {
  const config = useRuntimeConfig(event)
  return {
    baseUrl: (config.acceleratorBaseUrl as string).replace(/\/$/, ''),
    adminToken: config.acceleratorAdminToken as string,
  }
}

export async function acceleratorFetch<T>(
  event: H3Event,
  path: string,
  options: {
    method?: string
    body?: unknown
    query?: Record<string, string>
  } = {},
): Promise<T> {
  const { baseUrl, adminToken } = getAcceleratorConfig(event)
  const method = options.method || 'GET'

  const headers: Record<string, string> = {
    Accept: 'application/json',
  }
  if (adminToken) {
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
    const e = err as { statusCode?: number, data?: { error?: string }, message?: string }
    const statusCode = e.statusCode || 502
    const message = e.data?.error || e.message || 'Accelerator request failed'
    throw createError({
      statusCode,
      statusMessage: message,
      data: { error: message },
    })
  }
}