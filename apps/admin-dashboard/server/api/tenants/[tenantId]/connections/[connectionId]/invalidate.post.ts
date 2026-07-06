import type { InvalidateResponse } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const tenantId = getRouterParam(event, 'tenantId')
  const connectionId = getRouterParam(event, 'connectionId')
  if (!tenantId || !connectionId) {
    throw createError({ statusCode: 400, statusMessage: 'tenantId and connectionId required' })
  }
  const body = await readBody(event)
  return acceleratorFetch<InvalidateResponse>(
    event,
    `/api/v1/tenants/${encodeURIComponent(tenantId)}/connections/${encodeURIComponent(connectionId)}/invalidate`,
    { method: 'POST', body: body || {} },
  )
})
