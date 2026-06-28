export default defineEventHandler(async (event) => {
  const tenantId = getRouterParam(event, 'tenantId')
  const inviteId = getRouterParam(event, 'inviteId')
  return acceleratorFetch(
    event,
    `/api/v1/tenants/${encodeURIComponent(tenantId || '')}/invites/${encodeURIComponent(inviteId || '')}`,
    { method: 'DELETE' },
  )
})
