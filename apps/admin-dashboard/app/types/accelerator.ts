export interface Tenant {
  id: string
  name: string
  status: string
  created_at: string
  updated_at: string
}

export interface CollectionPolicy {
  enabled: boolean
  ttlSeconds: number
  maxResultBytes?: number | null
}

export interface CachePolicy {
  tenantId: string
  defaultTtlSeconds: number
  collections: Record<string, CollectionPolicy>
  cacheKeyVersion: number
  updatedAt: string
}

export interface Token {
  id: string
  tenantId: string
  description?: string
  created_at: string
  expires_at?: string | null
  revoked_at?: string | null
}

export interface IssueTokenResponse {
  tokenId: string
  rawToken: string
  tenantId: string
  description?: string
  createdAt: string
}

export interface StatusResponse {
  status: string
}

export interface InvalidateRequest {
  db?: string
  coll?: string
  tags?: string[]
}

export interface InvalidateResponse {
  status: string
  tenantId: string
  db?: string
  coll?: string
  tags?: string[]
}

export interface SavingsReport {
  tenantId: string
  note: string
  suggestedQueries: string[]
}

export interface ApiError {
  error: string
  statusCode?: number
}