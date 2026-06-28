export default defineEventHandler(async (event) => {
  const body = await readBody(event)
  return acceleratorFetch(event, '/api/v1/auth/verify', {
    method: 'POST',
    body,
    userAuth: false,
  })
})
