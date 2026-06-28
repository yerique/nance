export default defineEventHandler(async (event) => {
  return acceleratorFetch(event, '/api/v1/me/invites')
})
