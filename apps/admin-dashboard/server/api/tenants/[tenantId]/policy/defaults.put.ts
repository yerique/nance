import type { StatusResponse } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const tenantId = getRouterParam(event, 'tenantId')
  if (!tenantId) {
    throw createError({ statusCode: 400, statusMessage: 'tenantId required' })
  }
  const body = await readBody<{ defaultTtlSeconds: number }>(event)
  return acceleratorFetch<StatusResponse>(
    event,
    `/api/v1/tenants/${encodeURIComponent(tenantId)}/policy/defaults`,
    { method: 'PUT', body },
  )
})