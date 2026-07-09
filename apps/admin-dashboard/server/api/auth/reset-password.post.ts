import type { StatusResponse } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const body = await readBody<{ token?: string, password?: string }>(event)
  return acceleratorFetch<StatusResponse>(event, '/api/v1/auth/reset-password', {
    method: 'POST',
    body: body || {},
    userAuth: false,
  })
})
