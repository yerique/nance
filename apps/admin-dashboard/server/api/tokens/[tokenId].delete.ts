import type { StatusResponse } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  const tokenId = getRouterParam(event, 'tokenId')
  if (!tokenId) {
    throw createError({ statusCode: 400, statusMessage: 'tokenId required' })
  }
  return acceleratorFetch<StatusResponse>(
    event,
    `/api/v1/tokens/${encodeURIComponent(tokenId)}`,
    { method: 'DELETE' },
  )
})