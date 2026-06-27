import type { Token } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const tenantId = getRouterParam(event, 'tenantId')
  if (!tenantId) {
    throw createError({ statusCode: 400, statusMessage: 'tenantId required' })
  }
  return acceleratorFetch<Token[]>(
    event,
    `/api/v1/tenants/${encodeURIComponent(tenantId)}/tokens`,
  )
})