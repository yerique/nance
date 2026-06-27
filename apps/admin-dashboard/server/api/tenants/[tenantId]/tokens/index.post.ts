import type { IssueTokenResponse } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const tenantId = getRouterParam(event, 'tenantId')
  if (!tenantId) {
    throw createError({ statusCode: 400, statusMessage: 'tenantId required' })
  }
  const body = await readBody<{ description?: string }>(event).catch(() => ({}))
  return acceleratorFetch<IssueTokenResponse>(
    event,
    `/api/v1/tenants/${encodeURIComponent(tenantId)}/tokens`,
    { method: 'POST', body: body || {} },
  )
})