import type { StatusResponse } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const tenantId = getRouterParam(event, 'tenantId')
  const connectionId = getRouterParam(event, 'connectionId')
  const dbColl = getRouterParam(event, 'dbColl')
  if (!tenantId || !connectionId || !dbColl) {
    throw createError({ statusCode: 400, statusMessage: 'tenantId, connectionId, and dbColl required' })
  }
  const body = await readBody(event)
  return acceleratorFetch<StatusResponse>(
    event,
    `/api/v1/tenants/${encodeURIComponent(tenantId)}/connections/${encodeURIComponent(connectionId)}/policy/collections/${encodeURIComponent(dbColl)}`,
    { method: 'PUT', body },
  )
})
