import type { InvalidateRequest, InvalidateResponse } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const tenantId = getRouterParam(event, 'tenantId')
  if (!tenantId) {
    throw createError({ statusCode: 400, statusMessage: 'tenantId required' })
  }
  const body = await readBody<InvalidateRequest>(event).catch(() => ({} as InvalidateRequest))
  return acceleratorFetch<InvalidateResponse>(
    event,
    `/api/v1/tenants/${encodeURIComponent(tenantId)}/invalidate`,
    { method: 'POST', body: body || {} },
  )
})