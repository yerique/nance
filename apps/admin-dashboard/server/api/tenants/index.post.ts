import type { Tenant } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const body = await readBody<{ id: string, name: string }>(event)
  if (!body?.id || !body?.name) {
    throw createError({ statusCode: 400, statusMessage: 'id and name are required' })
  }
  return acceleratorFetch<Tenant>(event, '/api/v1/tenants', {
    method: 'POST',
    body,
  })
})