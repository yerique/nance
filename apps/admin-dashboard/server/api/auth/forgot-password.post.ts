import type { StatusResponse } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const body = await readBody<{ email?: string }>(event)
  return acceleratorFetch<StatusResponse>(event, '/api/v1/auth/forgot-password', {
    method: 'POST',
    body: body || {},
    userAuth: false,
  })
})
