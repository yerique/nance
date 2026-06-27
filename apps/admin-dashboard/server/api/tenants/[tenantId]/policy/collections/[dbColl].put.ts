import type { CollectionPolicy, StatusResponse } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const tenantId = getRouterParam(event, 'tenantId')
  const dbColl = getRouterParam(event, 'dbColl')
  if (!tenantId || !dbColl) {
    throw createError({ statusCode: 400, statusMessage: 'tenantId and dbColl required' })
  }
  const body = await readBody<CollectionPolicy>(event)
  return acceleratorFetch<StatusResponse>(
    event,
    `/api/v1/tenants/${encodeURIComponent(tenantId)}/policy/collections/${encodeURIComponent(dbColl)}`,
    { method: 'PUT', body },
  )
})