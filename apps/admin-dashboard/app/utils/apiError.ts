/**
 * Extract a human-readable message from $fetch / ofetch / h3 / FetchError shapes.
 *
 * Important: Nuxt/h3 error payloads often look like:
 *   { error: true, statusMessage: "...", message: "...", data: { error: "backend msg" } }
 * so `data.error` is a boolean, not the message. Prefer nested string fields first.
 */
export function apiErrorMessage(err: unknown): string {
  if (err == null) return 'Request failed'
  if (typeof err === 'string') {
    const s = err.trim()
    return s || 'Request failed'
  }
  if (err instanceof Error && !isFetchLike(err)) {
    const m = err.message?.trim()
    if (m && !isGenericFetchMessage(m)) return m
  }

  const e = err as Record<string, unknown>
  const data = asRecord(e.data) ?? asRecord((e as { response?: { _data?: unknown } }).response?._data)

  const candidates: unknown[] = [
    // Nested payload from our Nitro proxy: createError({ data: { error: "..." } })
    data && asRecord(data.data)?.error,
    data && asRecord(data.data)?.message,
    // Backend body when fetched directly: { error: "..." }
    data && stringOnly(data.error),
    data && stringOnly(data.message),
    data && stringOnly(data.statusMessage),
    // Top-level h3 / ofetch fields
    stringOnly(e.statusMessage),
    stringOnly(e.message),
    stringOnly(data?.statusMessage),
    // Rare: body is a plain string
    typeof data === 'string' ? data : undefined,
  ]

  for (const c of candidates) {
    const s = normalizeMessage(c)
    if (s) return s
  }

  // Last resort: ofetch often sets message like `[POST] "/api/...": 409 Conflict`
  const fallback = normalizeMessage(e.message) || normalizeMessage(e.statusMessage)
  if (fallback) return fallback

  const status = e.statusCode ?? e.status
  if (typeof status === 'number' && status > 0) {
    return `Request failed (${status})`
  }
  return 'Request failed'
}

function isFetchLike(err: Error): boolean {
  return 'data' in err || 'statusCode' in err || 'status' in err || err.name === 'FetchError'
}

function asRecord(v: unknown): Record<string, unknown> | undefined {
  if (v && typeof v === 'object' && !Array.isArray(v)) {
    return v as Record<string, unknown>
  }
  return undefined
}

/** Accept only real string messages — never boolean `true` from h3. */
function stringOnly(v: unknown): string | undefined {
  return typeof v === 'string' ? v : undefined
}

function normalizeMessage(v: unknown): string | undefined {
  if (typeof v !== 'string') return undefined
  const s = v.trim()
  if (!s || s === 'true' || s === 'false') return undefined
  return s
}

function isGenericFetchMessage(m: string): boolean {
  // ofetch default: [POST] "http://...": 409 Conflict
  return /^\[[A-Z]+\]\s+"/.test(m)
}
