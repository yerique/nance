import type { AuthVerifyResponse } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const body = await readBody<{ email?: string, password?: string }>(event)
  return acceleratorFetch<AuthVerifyResponse>(event, '/api/v1/auth/login-password', {
    method: 'POST',
    body: body || {},
    userAuth: false,
  })
})
