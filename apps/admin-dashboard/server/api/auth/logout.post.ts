export default defineEventHandler(async (event) => {
  return acceleratorFetch(event, '/api/v1/auth/logout', { method: 'POST' })
})
