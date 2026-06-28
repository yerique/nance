export default defineEventHandler(async (event) => {
  const tenantId = getRouterParam(event, 'tenantId')
  const userId = getRouterParam(event, 'userId')
  return acceleratorFetch(
    event,
    `/api/v1/tenants/${encodeURIComponent(tenantId || '')}/members/${encodeURIComponent(userId || '')}`,
    { method: 'DELETE' },
  )
})
