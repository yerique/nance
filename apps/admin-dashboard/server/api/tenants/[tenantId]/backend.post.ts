import type { StatusResponse } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const tenantId = getRouterParam(event, 'tenantId')
  if (!tenantId) {
    throw createError({ statusCode: 400, statusMessage: 'tenantId required' })
  }
  const body = await readBody<{ uri: string }>(event)
  if (!body?.uri) {
    throw createError({ statusCode: 400, statusMessage: 'uri is required' })
  }
  return acceleratorFetch<StatusResponse>(
    event,
    `/api/v1/tenants/${encodeURIComponent(tenantId)}/backend`,
    { method: 'POST', body },
  )
})