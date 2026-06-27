import type {
  CachePolicy,
  CollectionPolicy,
  InvalidateRequest,
  InvalidateResponse,
  IssueTokenResponse,
  SavingsReport,
  StatusResponse,
  Tenant,
  Token,
} from '~/types/accelerator'

function apiErrorMessage(err: unknown): string {
  const e = err as { data?: { error?: string, statusMessage?: string }, statusMessage?: string, message?: string }
  return e.data?.error || e.data?.statusMessage || e.statusMessage || e.message || 'Request failed'
}

export function useAcceleratorApi() {
  async function listTenants() {
    return $fetch<Tenant[]>('/api/tenants')
  }

  async function getTenant(tenantId: string) {
    return $fetch<Tenant>(`/api/tenants/${encodeURIComponent(tenantId)}`)
  }

  async function createTenant(id: string, name: string) {
    return $fetch<Tenant>('/api/tenants', {
      method: 'POST',
      body: { id, name },
    })
  }

  async function setBackend(tenantId: string, uri: string) {
    return $fetch<StatusResponse>(`/api/tenants/${encodeURIComponent(tenantId)}/backend`, {
      method: 'POST',
      body: { uri },
    })
  }

  async function testBackend(tenantId: string) {
    return $fetch<StatusResponse>(`/api/tenants/${encodeURIComponent(tenantId)}/backend/test`, {
      method: 'POST',
    })
  }

  async function getPolicy(tenantId: string) {
    return $fetch<CachePolicy>(`/api/tenants/${encodeURIComponent(tenantId)}/policy`)
  }

  async function setDefaultTtl(tenantId: string, defaultTtlSeconds: number) {
    return $fetch<StatusResponse>(`/api/tenants/${encodeURIComponent(tenantId)}/policy/defaults`, {
      method: 'PUT',
      body: { defaultTtlSeconds },
    })
  }

  async function setCollectionPolicy(tenantId: string, dbColl: string, policy: CollectionPolicy) {
    return $fetch<StatusResponse>(
      `/api/tenants/${encodeURIComponent(tenantId)}/policy/collections/${encodeURIComponent(dbColl)}`,
      { method: 'PUT', body: policy },
    )
  }

  async function listTokens(tenantId: string) {
    return $fetch<Token[]>(`/api/tenants/${encodeURIComponent(tenantId)}/tokens`)
  }

  async function issueToken(tenantId: string, description?: string) {
    return $fetch<IssueTokenResponse>(`/api/tenants/${encodeURIComponent(tenantId)}/tokens`, {
      method: 'POST',
      body: { description },
    })
  }

  async function revokeToken(tokenId: string) {
    return $fetch<StatusResponse>(`/api/tokens/${encodeURIComponent(tokenId)}`, {
      method: 'DELETE',
    })
  }

  async function invalidate(tenantId: string, req: InvalidateRequest = {}) {
    return $fetch<InvalidateResponse>(`/api/tenants/${encodeURIComponent(tenantId)}/invalidate`, {
      method: 'POST',
      body: req,
    })
  }

  async function getSavings(tenantId: string) {
    return $fetch<SavingsReport>(`/api/tenants/${encodeURIComponent(tenantId)}/savings`)
  }

  async function checkHealth() {
    return $fetch<{ ok: boolean, accelerator: string, health: string | null }>('/api/health')
  }

  return {
    listTenants,
    getTenant,
    createTenant,
    setBackend,
    testBackend,
    getPolicy,
    setDefaultTtl,
    setCollectionPolicy,
    listTokens,
    issueToken,
    revokeToken,
    invalidate,
    getSavings,
    checkHealth,
    apiErrorMessage,
  }
}