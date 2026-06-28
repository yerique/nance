export default defineEventHandler(async (event) => {
  const body = await readBody(event)
  return acceleratorFetch(event, '/api/v1/me', { method: 'PATCH', body })
})
