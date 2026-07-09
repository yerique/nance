import type { User } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const body = await readBody<{ currentPassword?: string, password?: string }>(event)
  return acceleratorFetch<User>(event, '/api/v1/me/password', {
    method: 'PUT',
    body: body || {},
  })
})
