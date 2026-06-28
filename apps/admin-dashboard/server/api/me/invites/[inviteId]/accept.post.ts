export default defineEventHandler(async (event) => {
  const inviteId = getRouterParam(event, 'inviteId')
  return acceleratorFetch(event, `/api/v1/me/invites/${encodeURIComponent(inviteId || '')}/accept`, {
    method: 'POST',
  })
})
