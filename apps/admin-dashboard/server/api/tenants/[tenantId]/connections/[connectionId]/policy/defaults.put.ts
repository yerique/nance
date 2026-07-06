import type { StatusResponse } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const tenantId = getRouterParam(event, 'tenantId')
  const connectionId = getRouterParam(event, 'connectionId')
  if (!tenantId || !connectionId) {
    throw createError({ statusCode: 400, statusMessage: 'tenantId and connectionId required' })
  }
  const body = await readBody(event)
  return acceleratorFetch<StatusResponse>(
    event,
    `/api/v1/tenants/${encodeURIComponent(tenantId)}/connections/${encodeURIComponent(connectionId)}/policy/defaults`,
    { method: 'PUT', body },
  )
})
