import type { Tenant } from '~/types/accelerator'

export default defineEventHandler(async (event) => {
  return acceleratorFetch<Tenant[]>(event, '/api/v1/tenants')
})