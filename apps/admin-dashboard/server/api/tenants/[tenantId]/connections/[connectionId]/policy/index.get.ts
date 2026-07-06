import type { CachePolicy } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const tenantId = getRouterParam(event, 'tenantId')
  const connectionId = getRouterParam(event, 'connectionId')
  if (!tenantId || !connectionId) {
    throw createError({ statusCode: 400, statusMessage: 'tenantId and connectionId required' })
  }
  return acceleratorFetch<CachePolicy>(
    event,
    `/api/v1/tenants/${encodeURIComponent(tenantId)}/connections/${encodeURIComponent(connectionId)}/policy`,
  )
})
