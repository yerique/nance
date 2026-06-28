<script setup lang="ts">
import type {
  CachePolicy,
  CollectionPolicy,
  IssueTokenResponse,
  OrganizationInvite,
  OrganizationMember,
  SavingsReport,
  Tenant,
  Token,
} from '~/types/accelerator'

const route = useRoute()
const api = useAcceleratorApi()
const tenantId = computed(() => String(route.params.id || ''))

const tab = ref<'overview' | 'backend' | 'cache' | 'tokens' | 'members' | 'invalidate' | 'savings'>('overview')
const tabs = [
  { id: 'overview' as const, label: 'Overview' },
  { id: 'backend' as const, label: 'Connection' },
  { id: 'cache' as const, label: 'Caching' },
  { id: 'tokens' as const, label: 'Tokens' },
  { id: 'members' as const, label: 'Members' },
  { id: 'invalidate' as const, label: 'Invalidate' },
  { id: 'savings' as const, label: 'Savings' },
]

const tenant = ref<Tenant | null>(null)
const policy = ref<CachePolicy | null>(null)
const tokens = ref<Token[]>([])
const savings = ref<SavingsReport | null>(null)
const loading = ref(true)
const error = ref('')
const flash = ref<{ type: 'success' | 'error' | 'info' | 'warning', msg: string } | null>(null)

// Backend form
const backendUri = ref('')
const backendBusy = ref(false)

// Cache defaults
const defaultTtl = ref(60)
const defaultsBusy = ref(false)

// Per-collection TTL overrides (real collection name, not *_cache)
const newCollKey = ref('')
const newCollTtl = ref(60)
const newCollMaxBytes = ref<number | undefined>(undefined)
const collBusy = ref(false)

// Tokens
const tokenDesc = ref('')
const tokenBusy = ref(false)
const issuedToken = ref<IssueTokenResponse | null>(null)

// Invalidate
const invDb = ref('')
const invColl = ref('')
const invTags = ref('')
const invBusy = ref(false)

// Members
const members = ref<OrganizationMember[]>([])
const pendingInvites = ref<OrganizationInvite[]>([])
const inviteEmail = ref('')
const inviteRole = ref<'member' | 'admin' | 'owner'>('member')
const membersBusy = ref(false)

function showFlash(type: 'success' | 'error' | 'info' | 'warning', msg: string) {
  flash.value = { type, msg }
  setTimeout(() => {
    if (flash.value?.msg === msg) flash.value = null
  }, 6000)
}

async function loadTenant() {
  loading.value = true
  error.value = ''
  try {
    tenant.value = await api.getTenant(tenantId.value)
  }
  catch (e) {
    error.value = api.apiErrorMessage(e)
    tenant.value = null
  }
  finally {
    loading.value = false
  }
}

async function loadPolicy() {
  try {
    policy.value = await api.getPolicy(tenantId.value)
    defaultTtl.value = policy.value.defaultTtlSeconds ?? 60
  }
  catch (e) {
    showFlash('error', `Policy: ${api.apiErrorMessage(e)}`)
  }
}

async function loadTokens() {
  try {
    tokens.value = await api.listTokens(tenantId.value)
  }
  catch (e) {
    showFlash('error', `Tokens: ${api.apiErrorMessage(e)}`)
  }
}

async function loadSavings() {
  try {
    savings.value = await api.getSavings(tenantId.value)
  }
  catch (e) {
    showFlash('error', `Savings: ${api.apiErrorMessage(e)}`)
  }
}

async function loadMembers() {
  try {
    const [m, inv] = await Promise.all([
      api.listMembers(tenantId.value),
      api.listTenantInvites(tenantId.value).catch(() => [] as OrganizationInvite[]),
    ])
    members.value = m
    pendingInvites.value = inv
  }
  catch (e) {
    showFlash('error', `Members: ${api.apiErrorMessage(e)}`)
  }
}

async function sendInvite() {
  if (!inviteEmail.value.trim()) return
  membersBusy.value = true
  try {
    await api.inviteMember(tenantId.value, inviteEmail.value.trim(), inviteRole.value)
    inviteEmail.value = ''
    showFlash('success', 'Invite sent')
    await loadMembers()
  }
  catch (e) {
    showFlash('error', api.apiErrorMessage(e))
  }
  finally {
    membersBusy.value = false
  }
}

async function onRemoveMember(userId: string) {
  membersBusy.value = true
  try {
    await api.removeMember(tenantId.value, userId)
    showFlash('success', 'Member removed')
    await loadMembers()
  }
  catch (e) {
    showFlash('error', api.apiErrorMessage(e))
  }
  finally {
    membersBusy.value = false
  }
}

async function onRevokeInvite(inviteId: string) {
  membersBusy.value = true
  try {
    await api.revokeInvite(tenantId.value, inviteId)
    showFlash('success', 'Invite revoked')
    await loadMembers()
  }
  catch (e) {
    showFlash('error', api.apiErrorMessage(e))
  }
  finally {
    membersBusy.value = false
  }
}

watch(tab, async (t) => {
  if (t === 'cache' && !policy.value) await loadPolicy()
  if (t === 'tokens') await loadTokens()
  if (t === 'savings') await loadSavings()
  if (t === 'members') await loadMembers()
})

onMounted(async () => {
  await loadTenant()
  // Prefetch policy so overview shows active default TTL
  await loadPolicy()
})

// —— Backend ——
async function saveBackend() {
  if (!backendUri.value.trim()) {
    showFlash('error', 'MongoDB URI is required')
    return
  }
  backendBusy.value = true
  try {
    await api.setBackend(tenantId.value, backendUri.value.trim())
    backendUri.value = ''
    showFlash('success', 'Backend URI stored (encrypted at rest). Never shown again via API.')
  }
  catch (e) {
    showFlash('error', api.apiErrorMessage(e))
  }
  finally {
    backendBusy.value = false
  }
}

async function testBackend() {
  backendBusy.value = true
  try {
    const res = await api.testBackend(tenantId.value)
    showFlash('success', res.status || 'Connection successful')
  }
  catch (e) {
    showFlash('error', api.apiErrorMessage(e))
  }
  finally {
    backendBusy.value = false
  }
}

// —— Cache ——
async function saveDefaults() {
  defaultsBusy.value = true
  try {
    const ttl = Number(defaultTtl.value)
    if (!ttl || ttl < 1) {
      showFlash('error', 'Default TTL must be at least 1 second')
      return
    }
    await api.setDefaultTtl(tenantId.value, ttl)
    await loadPolicy()
    showFlash('success', `Default cache TTL set to ${ttl}s for all _cache queries`)
  }
  catch (e) {
    showFlash('error', api.apiErrorMessage(e))
  }
  finally {
    defaultsBusy.value = false
  }
}

async function upsertCollection(key: string, pol: CollectionPolicy) {
  collBusy.value = true
  try {
    await api.setCollectionPolicy(tenantId.value, key, pol)
    await loadPolicy()
    showFlash('success', `Override saved for ${key} (applies when clients use ${key.split('.').pop()}_cache)`)
  }
  catch (e) {
    showFlash('error', api.apiErrorMessage(e))
  }
  finally {
    collBusy.value = false
  }
}

async function addCollection() {
  const key = newCollKey.value.trim()
  if (!key || !key.includes('.')) {
    showFlash('error', 'Use real db.collection format (e.g. mydb.orders), not mydb.orders_cache')
    return
  }
  if (key.endsWith('_cache')) {
    showFlash('error', 'Use the real collection name (without _cache). Clients append _cache in queries.')
    return
  }
  const ttl = Number(newCollTtl.value) || Number(defaultTtl.value) || 60
  const pol: CollectionPolicy = {
    enabled: true,
    ttlSeconds: ttl,
  }
  if (newCollMaxBytes.value && newCollMaxBytes.value > 0) {
    pol.maxResultBytes = Number(newCollMaxBytes.value)
  }
  await upsertCollection(key, pol)
  newCollKey.value = ''
  newCollMaxBytes.value = undefined
}

async function removeCollectionOverride(key: string) {
  // API has no delete; TTL 0 means inherit organization default in the proxy.
  collBusy.value = true
  try {
    await api.setCollectionPolicy(tenantId.value, key, { enabled: true, ttlSeconds: 0 })
    await loadPolicy()
    showFlash('info', `${key} will inherit the organization default TTL (${defaultTtl.value}s)`)
  }
  catch (e) {
    showFlash('error', api.apiErrorMessage(e))
  }
  finally {
    collBusy.value = false
  }
}

const collectionEntries = computed(() => {
  if (!policy.value?.collections) return []
  return Object.entries(policy.value.collections).map(([key, pol]) => ({ key, ...pol }))
})

function effectiveTtl(row: { ttlSeconds?: number }) {
  if (row.ttlSeconds && row.ttlSeconds > 0) return row.ttlSeconds
  return policy.value?.defaultTtlSeconds ?? defaultTtl.value ?? 60
}

// —— Tokens ——
async function issueToken() {
  tokenBusy.value = true
  issuedToken.value = null
  try {
    issuedToken.value = await api.issueToken(tenantId.value, tokenDesc.value.trim() || undefined)
    tokenDesc.value = ''
    await loadTokens()
    showFlash('warning', 'Copy the raw token now — it is only shown once.')
  }
  catch (e) {
    showFlash('error', api.apiErrorMessage(e))
  }
  finally {
    tokenBusy.value = false
  }
}

async function revokeToken(tokenId: string) {
  if (!confirm('Revoke this token? Clients using it will fail auth immediately.')) return
  try {
    await api.revokeToken(tokenId)
    await loadTokens()
    showFlash('success', 'Token revoked')
  }
  catch (e) {
    showFlash('error', api.apiErrorMessage(e))
  }
}

function copyText(text: string) {
  navigator.clipboard?.writeText(text).then(() => showFlash('info', 'Copied to clipboard'))
}

// —— Invalidate ——
async function runInvalidate() {
  invBusy.value = true
  try {
    const tags = invTags.value
      .split(',')
      .map(t => t.trim())
      .filter(Boolean)
    const res = await api.invalidate(tenantId.value, {
      db: invDb.value.trim() || undefined,
      coll: invColl.value.trim() || undefined,
      tags: tags.length ? tags : undefined,
    })
    showFlash('success', `Invalidated (tenant=${res.tenantId}${res.db ? `, db=${res.db}` : ''}${res.coll ? `, coll=${res.coll}` : ''})`)
  }
  catch (e) {
    showFlash('error', api.apiErrorMessage(e))
  }
  finally {
    invBusy.value = false
  }
}
</script>

<template>
  <div class="page">
    <div class="breadcrumb">
      <NuxtLink to="/">Organizations</NuxtLink>
      <span>/</span>
      <span class="mono">{{ tenantId }}</span>
    </div>

    <div class="page-header">
      <div>
        <h2>{{ tenant?.name || tenantId }}</h2>
        <p class="subtitle">Organization · <span class="mono">{{ tenantId }}</span></p>
      </div>
      <span v-if="tenant" :class="statusBadgeClass(tenant.status)">{{ tenant.status }}</span>
    </div>

    <div v-if="flash" class="alert" :class="`alert-${flash.type}`">{{ flash.msg }}</div>
    <div v-if="error" class="alert alert-error">{{ error }}</div>
    <div v-if="loading" class="loading">Loading tenant…</div>

    <template v-else-if="tenant">
      <div class="tabs">
        <button
          v-for="t in tabs"
          :key="t.id"
          type="button"
          class="tab"
          :class="{ active: tab === t.id }"
          @click="tab = t.id"
        >
          {{ t.label }}
        </button>
      </div>

      <!-- Overview -->
      <div v-if="tab === 'overview'" class="stats-row">
        <div class="stat-card">
          <div class="stat-label">Tenant ID</div>
          <div class="stat-value" style="font-size: 1rem; font-family: var(--font-mono);">{{ tenant.id }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Status</div>
          <div class="stat-value" style="font-size: 1.1rem;">{{ tenant.status }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Created</div>
          <div class="stat-value" style="font-size: 0.95rem;">{{ formatDate(tenant.created_at) }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Updated</div>
          <div class="stat-value" style="font-size: 0.95rem;">{{ formatDate(tenant.updated_at) }}</div>
        </div>
      </div>

      <div v-if="tab === 'overview'" class="card">
        <h3 class="card-title">How caching works</h3>
        <p class="card-desc">
          Clients opt in <strong>per query</strong> by using a collection name that ends with
          <code class="mono">_cache</code>. The proxy strips that suffix, reads the real collection,
          and serves results from Redis with a default TTL of
          <strong>{{ policy?.defaultTtlSeconds ?? defaultTtl }} seconds</strong>
          (override under <strong>Caching</strong>). Without the suffix, every query hits MongoDB.
        </p>
        <ul class="help-list">
          <li><code class="mono">db.orders_cache.find(…)</code> — cached (real collection: <code class="mono">orders</code>)</li>
          <li><code class="mono">db.orders.find(…)</code> — always bypasses cache</li>
          <li>Writes to <code class="mono">orders</code> invalidate cached entries for that namespace</li>
        </ul>
        <div class="form-actions">
          <button class="btn btn-secondary" type="button" @click="tab = 'backend'">Configure backend</button>
          <button class="btn btn-secondary" type="button" @click="tab = 'cache'">Configure caching</button>
          <button class="btn btn-secondary" type="button" @click="tab = 'tokens'">Issue token</button>
          <button class="btn btn-secondary" type="button" @click="tab = 'members'">Manage members</button>
          <button class="btn btn-secondary" type="button" @click="tab = 'invalidate'">Invalidate cache</button>
        </div>
      </div>

      <!-- Backend / Connection -->
      <div v-if="tab === 'backend'" class="card">
        <h3 class="card-title">MongoDB backend connection</h3>
        <p class="card-desc">
          Store this organization's real MongoDB URI. It is encrypted at rest with
          <code class="mono">NANCE_MASTER_KEY</code> and never returned by the API.
        </p>
        <div class="form-row">
          <label for="backend-uri">Connection URI</label>
          <input
            id="backend-uri"
            v-model="backendUri"
            class="mono"
            type="password"
            placeholder="mongodb://user:pass@host:27017/db?…"
            autocomplete="off"
          >
          <span class="hint">Paste a full MongoDB connection string. Leave blank and use Test if already configured.</span>
        </div>
        <div class="form-actions">
          <button class="btn btn-primary" type="button" :disabled="backendBusy" @click="saveBackend">
            {{ backendBusy ? 'Saving…' : 'Save encrypted URI' }}
          </button>
          <button class="btn btn-secondary" type="button" :disabled="backendBusy" @click="testBackend">
            Test connection
          </button>
        </div>
      </div>

      <!-- Caching -->
      <template v-if="tab === 'cache'">
        <div class="card callout-card">
          <h3 class="card-title">Opt-in with <code class="mono">_cache</code></h3>
          <p class="card-desc" style="margin-bottom: 0.75rem;">
            Every collection is eligible for caching. Developers choose per query by appending
            <code class="mono">_cache</code> to the collection name. No policy toggle is required to turn caching on.
          </p>
          <div class="code-examples">
            <div class="code-example">
              <span class="badge badge-success">cached</span>
              <code class="mono">db.orders_cache.find(&#123; status: "open" &#125;)</code>
              <span class="text-dim text-sm">→ real <code class="mono">orders</code> · TTL {{ policy?.defaultTtlSeconds ?? defaultTtl }}s unless overridden</span>
            </div>
            <div class="code-example">
              <span class="badge badge-muted">bypass</span>
              <code class="mono">db.orders.find(&#123; status: "open" &#125;)</code>
              <span class="text-dim text-sm">→ always MongoDB, never Redis</span>
            </div>
          </div>
        </div>

        <div class="card">
          <h3 class="card-title">Default TTL</h3>
          <p class="card-desc">
            Applied to <strong>all</strong> <code class="mono">*_cache</code> queries for this organization
            unless a per-collection override is set below. Platform default is <strong>60 seconds</strong>.
          </p>
          <div class="inline-form">
            <div class="form-row">
              <label for="default-ttl">Default TTL (seconds)</label>
              <input id="default-ttl" v-model.number="defaultTtl" type="number" min="1" step="1">
              <span class="hint">Example: 60 caches results for one minute after each miss.</span>
            </div>
            <button class="btn btn-primary" type="button" :disabled="defaultsBusy" @click="saveDefaults">
              {{ defaultsBusy ? 'Saving…' : 'Save default TTL' }}
            </button>
          </div>
          <p v-if="policy" class="text-dim text-sm mt-2">
            Active default: <strong>{{ policy.defaultTtlSeconds }}s</strong>
            · Cache key version: {{ policy.cacheKeyVersion }}
            · Updated {{ formatDate(policy.updatedAt) }}
          </p>
        </div>

        <div class="card">
          <h3 class="card-title">Per-collection overrides</h3>
          <p class="card-desc">
            Optional. Use the <strong>real</strong> collection name (<code class="mono">db.orders</code>, not
            <code class="mono">db.orders_cache</code>) to set a different TTL or max cached result size for that namespace.
            Leave empty to use the organization default ({{ policy?.defaultTtlSeconds ?? defaultTtl }}s) for every collection.
          </p>

          <div v-if="!collectionEntries.length" class="empty-state" style="padding: 1.5rem;">
            <p><strong>No overrides</strong> — all <code class="mono">*_cache</code> queries use the default TTL above.</p>
            <p class="text-sm text-muted">Add an override only when a hot collection needs a shorter or longer TTL.</p>
          </div>

          <div v-else class="table-wrap mb-2">
            <table class="data-table">
              <thead>
                <tr>
                  <th>Real collection</th>
                  <th>Client uses</th>
                  <th>TTL (s)</th>
                  <th>Max result bytes</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in collectionEntries" :key="row.key">
                  <td class="mono">{{ row.key }}</td>
                  <td class="mono text-sm">{{ row.key }}_cache</td>
                  <td>
                    <strong>{{ effectiveTtl(row) }}</strong>
                    <span v-if="!row.ttlSeconds || row.ttlSeconds <= 0" class="text-dim text-sm"> (default)</span>
                  </td>
                  <td class="text-muted">{{ row.maxResultBytes ?? 'default (1 MiB)' }}</td>
                  <td>
                    <button
                      class="btn btn-ghost btn-sm"
                      type="button"
                      :disabled="collBusy"
                      @click="removeCollectionOverride(row.key)"
                    >
                      Use default TTL
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <h4 class="text-sm text-muted mb-1" style="font-weight: 600;">Add / update override</h4>
          <div class="inline-form">
            <div class="form-row">
              <label>Real db.collection</label>
              <input v-model="newCollKey" class="mono" placeholder="mydb.orders">
              <span class="hint">Not <code class="mono">mydb.orders_cache</code></span>
            </div>
            <div class="form-row">
              <label>TTL (s)</label>
              <input v-model.number="newCollTtl" type="number" min="1" style="max-width: 100px;" :placeholder="String(defaultTtl)">
            </div>
            <div class="form-row">
              <label>Max bytes (optional)</label>
              <input v-model.number="newCollMaxBytes" type="number" min="1" style="max-width: 120px;" placeholder="1048576">
            </div>
            <button class="btn btn-primary" type="button" :disabled="collBusy" @click="addCollection">
              Save override
            </button>
          </div>
        </div>
      </template>

      <!-- Tokens -->
      <template v-if="tab === 'tokens'">
        <div class="card">
          <h3 class="card-title">Issue access token</h3>
          <p class="card-desc">
            Tokens authenticate clients to the data-plane proxy (username = tenant ID, password = raw token,
            <code class="mono">authMechanism=PLAIN</code>). The raw secret is returned only once.
          </p>
          <div class="inline-form">
            <div class="form-row">
              <label>Description (optional)</label>
              <input v-model="tokenDesc" placeholder="ci-bot, local-dev, …">
            </div>
            <button class="btn btn-primary" type="button" :disabled="tokenBusy" @click="issueToken">
              {{ tokenBusy ? 'Issuing…' : 'Issue token' }}
            </button>
          </div>

          <div v-if="issuedToken" class="token-reveal">
            <div class="label">Raw token — copy now, shown only once</div>
            <code>{{ issuedToken.rawToken }}</code>
            <div class="form-actions mt-1">
              <button class="btn btn-secondary btn-sm" type="button" @click="copyText(issuedToken!.rawToken)">
                Copy token
              </button>
              <button
                class="btn btn-ghost btn-sm"
                type="button"
                @click="copyText(`mongodb://${tenantId}:${issuedToken!.rawToken}@127.0.0.1:27018/mydb?authMechanism=PLAIN&authSource=$external`)"
              >
                Copy sample URI
              </button>
            </div>
          </div>
        </div>

        <div class="card">
          <h3 class="card-title">Issued tokens</h3>
          <div v-if="!tokens.length" class="empty-state" style="padding: 1.5rem;">
            <p>No tokens yet.</p>
          </div>
          <div v-else class="table-wrap">
            <table class="data-table">
              <thead>
                <tr>
                  <th>Token ID</th>
                  <th>Description</th>
                  <th>Created</th>
                  <th>Revoked</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <tr v-for="tok in tokens" :key="tok.id">
                  <td class="mono text-sm">{{ tok.id }}</td>
                  <td>{{ tok.description || '—' }}</td>
                  <td class="text-muted text-sm">{{ formatDate(tok.created_at) }}</td>
                  <td>
                    <span v-if="tok.revoked_at" class="badge badge-danger">revoked</span>
                    <span v-else class="badge badge-success">active</span>
                  </td>
                  <td>
                    <button
                      v-if="!tok.revoked_at"
                      class="btn btn-danger btn-sm"
                      type="button"
                      @click="revokeToken(tok.id)"
                    >
                      Revoke
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>

      <!-- Members -->
      <div v-if="tab === 'members'" class="card">
        <h3 class="card-title">User management</h3>
        <p class="card-desc">Invite teammates by email. They sign in with the same email and accept the invite from Organizations.</p>
        <div class="grid-2" style="align-items: end;">
          <div class="form-row">
            <label>Email</label>
            <input v-model="inviteEmail" type="email" placeholder="teammate@company.com">
          </div>
          <div class="form-row">
            <label>Role</label>
            <select v-model="inviteRole">
              <option value="member">member</option>
              <option value="admin">admin</option>
              <option value="owner">owner</option>
            </select>
          </div>
        </div>
        <div class="form-actions">
          <button class="btn btn-primary" type="button" :disabled="membersBusy || !inviteEmail.trim()" @click="sendInvite">
            {{ membersBusy ? 'Working…' : 'Send invite' }}
          </button>
        </div>

        <h4 class="mt-3">Members</h4>
        <div class="table-wrap">
          <table class="data-table">
            <thead>
              <tr>
                <th>Email</th>
                <th>Name</th>
                <th>Role</th>
                <th />
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in members" :key="m.userId">
                <td>{{ m.email || m.userId }}</td>
                <td>{{ m.name || '—' }}</td>
                <td><span class="badge">{{ m.role }}</span></td>
                <td>
                  <button class="btn btn-danger btn-sm" type="button" :disabled="membersBusy" @click="onRemoveMember(m.userId)">
                    Remove
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <template v-if="pendingInvites.length">
          <h4 class="mt-3">Pending invites</h4>
          <div class="table-wrap">
            <table class="data-table">
              <thead>
                <tr>
                  <th>Email</th>
                  <th>Role</th>
                  <th>Expires</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <tr v-for="inv in pendingInvites" :key="inv.id">
                  <td>{{ inv.email }}</td>
                  <td>{{ inv.role }}</td>
                  <td class="text-sm text-muted">{{ inv.expires_at }}</td>
                  <td>
                    <button class="btn btn-ghost btn-sm" type="button" :disabled="membersBusy" @click="onRevokeInvite(inv.id)">
                      Revoke
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </template>
      </div>

      <!-- Invalidate -->
      <div v-if="tab === 'invalidate'" class="card">
        <h3 class="card-title">Explicit cache invalidation</h3>
        <p class="card-desc">
          Flush Redis entries for this organization. Use the <strong>real</strong> collection name
          (e.g. <code class="mono">orders</code>), matching what was stored from <code class="mono">orders_cache</code> reads.
          Writes through the proxy already invalidate that namespace automatically.
        </p>
        <div class="grid-2">
          <div class="form-row">
            <label>Database (optional)</label>
            <input v-model="invDb" class="mono" placeholder="mydb">
          </div>
          <div class="form-row">
            <label>Real collection (optional)</label>
            <input v-model="invColl" class="mono" placeholder="orders">
            <span class="hint">Not <code class="mono">orders_cache</code></span>
          </div>
        </div>
        <div class="form-row">
          <label>Tags (optional, comma-separated)</label>
          <input v-model="invTags" class="mono" placeholder="user:1, order:99">
        </div>
        <div class="form-actions">
          <button class="btn btn-danger" type="button" :disabled="invBusy" @click="runInvalidate">
            {{ invBusy ? 'Invalidating…' : 'Invalidate cache' }}
          </button>
        </div>
      </div>

      <!-- Savings -->
      <div v-if="tab === 'savings'" class="card">
        <h3 class="card-title">Savings / metrics</h3>
        <div v-if="!savings" class="loading">Loading…</div>
        <template v-else>
          <p class="card-desc">{{ savings.note }}</p>
          <p class="text-sm text-muted mb-1">Suggested Prometheus queries:</p>
          <ul style="margin: 0; padding-left: 1.2rem;">
            <li v-for="(q, i) in savings.suggestedQueries" :key="i" class="mb-1">
              <code class="mono text-sm" style="word-break: break-all;">{{ q }}</code>
            </li>
          </ul>
        </template>
      </div>
    </template>
  </div>
</template>

<style scoped>
.help-list {
  margin: 0 0 1rem;
  padding-left: 1.2rem;
  color: var(--text-muted, #8b9bb0);
  font-size: 0.9rem;
  line-height: 1.55;
}
.help-list li { margin-bottom: 0.35rem; }
.callout-card {
  border-color: rgba(61, 156, 240, 0.35);
  background: linear-gradient(135deg, rgba(61, 156, 240, 0.08), transparent);
}
.code-examples {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.code-example {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 0.75rem;
  padding: 0.55rem 0.75rem;
  border-radius: 6px;
  background: var(--bg, #0b0f14);
  border: 1px solid var(--border-subtle, #1a2433);
}
.code-example code { font-size: 0.82rem; }
</style>