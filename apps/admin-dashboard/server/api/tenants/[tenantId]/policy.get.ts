import type { CachePolicy } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const tenantId = getRouterParam(event, 'tenantId')
  if (!tenantId) {
    throw createError({ statusCode: 400, statusMessage: 'tenantId required' })
  }
  return acceleratorFetch<CachePolicy>(
    event,
    `/api/v1/tenants/${encodeURIComponent(tenantId)}/policy`,
  )
})