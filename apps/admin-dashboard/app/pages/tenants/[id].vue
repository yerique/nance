<script setup lang="ts">
import type {
  CachePolicy,
  CollectionPolicy,
  IssueTokenResponse,
  SavingsReport,
  Tenant,
  Token,
} from '~/types/accelerator'

const route = useRoute()
const api = useAcceleratorApi()
const tenantId = computed(() => String(route.params.id || ''))

const tab = ref<'overview' | 'backend' | 'cache' | 'tokens' | 'invalidate' | 'savings'>('overview')
const tabs = [
  { id: 'overview' as const, label: 'Overview' },
  { id: 'backend' as const, label: 'Connection' },
  { id: 'cache' as const, label: 'Cache policy' },
  { id: 'tokens' as const, label: 'Tokens' },
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

// New collection policy
const newCollKey = ref('')
const newCollEnabled = ref(true)
const newCollTtl = ref(60)
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

watch(tab, async (t) => {
  if (t === 'cache' && !policy.value) await loadPolicy()
  if (t === 'tokens') await loadTokens()
  if (t === 'savings') await loadSavings()
})

onMounted(async () => {
  await loadTenant()
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
    await api.setDefaultTtl(tenantId.value, Number(defaultTtl.value))
    await loadPolicy()
    showFlash('success', 'Default TTL updated')
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
    showFlash('success', `Updated policy for ${key}`)
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
    showFlash('error', 'Use db.collection format (e.g. mydb.orders)')
    return
  }
  await upsertCollection(key, {
    enabled: newCollEnabled.value,
    ttlSeconds: Number(newCollTtl.value) || 60,
  })
  newCollKey.value = ''
}

const collectionEntries = computed(() => {
  if (!policy.value?.collections) return []
  return Object.entries(policy.value.collections).map(([key, pol]) => ({ key, ...pol }))
})

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
      <NuxtLink to="/">Tenants</NuxtLink>
      <span>/</span>
      <span class="mono">{{ tenantId }}</span>
    </div>

    <div class="page-header">
      <div>
        <h2>{{ tenant?.name || tenantId }}</h2>
        <p class="subtitle mono">{{ tenantId }}</p>
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
        <h3 class="card-title">Quick actions</h3>
        <div class="form-actions">
          <button class="btn btn-secondary" type="button" @click="tab = 'backend'">Configure backend</button>
          <button class="btn btn-secondary" type="button" @click="tab = 'cache'">Edit cache policy</button>
          <button class="btn btn-secondary" type="button" @click="tab = 'tokens'">Issue token</button>
          <button class="btn btn-secondary" type="button" @click="tab = 'invalidate'">Invalidate cache</button>
        </div>
      </div>

      <!-- Backend / Connection -->
      <div v-if="tab === 'backend'" class="card">
        <h3 class="card-title">MongoDB backend connection</h3>
        <p class="card-desc">
          Store the tenant's real MongoDB URI. It is encrypted at rest with
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

      <!-- Cache policy -->
      <template v-if="tab === 'cache'">
        <div class="card">
          <h3 class="card-title">Default TTL</h3>
          <p class="card-desc">Applied when a collection policy does not override TTL (seconds).</p>
          <div class="inline-form">
            <div class="form-row">
              <label for="default-ttl">Default TTL (seconds)</label>
              <input id="default-ttl" v-model.number="defaultTtl" type="number" min="1" step="1">
            </div>
            <button class="btn btn-primary" type="button" :disabled="defaultsBusy" @click="saveDefaults">
              {{ defaultsBusy ? 'Saving…' : 'Save defaults' }}
            </button>
          </div>
          <p v-if="policy" class="text-dim text-sm mt-2">
            Cache key version: {{ policy.cacheKeyVersion }} · Updated {{ formatDate(policy.updatedAt) }}
          </p>
        </div>

        <div class="card">
          <h3 class="card-title">Collection policies</h3>
          <p class="card-desc">
            Only collections with <strong>enabled: true</strong> participate in read-through caching (Phase 2).
            Key format: <code class="mono">db.collection</code>
          </p>

          <div v-if="!collectionEntries.length" class="empty-state" style="padding: 1.5rem;">
            <p>No per-collection policies yet. Add one below to enable caching.</p>
          </div>

          <div v-else class="table-wrap mb-2">
            <table class="data-table">
              <thead>
                <tr>
                  <th>Collection</th>
                  <th>Enabled</th>
                  <th>TTL (s)</th>
                  <th>Max result bytes</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in collectionEntries" :key="row.key">
                  <td class="mono">{{ row.key }}</td>
                  <td>
                    <span :class="row.enabled ? 'badge badge-success' : 'badge badge-muted'">
                      {{ row.enabled ? 'on' : 'off' }}
                    </span>
                  </td>
                  <td>{{ row.ttlSeconds }}</td>
                  <td class="text-muted">{{ row.maxResultBytes ?? '—' }}</td>
                  <td>
                    <button
                      class="btn btn-ghost btn-sm"
                      type="button"
                      :disabled="collBusy"
                      @click="upsertCollection(row.key, { enabled: !row.enabled, ttlSeconds: row.ttlSeconds, maxResultBytes: row.maxResultBytes })"
                    >
                      {{ row.enabled ? 'Disable' : 'Enable' }}
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <h4 class="text-sm text-muted mb-1" style="font-weight: 600;">Add / update collection</h4>
          <div class="inline-form">
            <div class="form-row">
              <label>db.collection</label>
              <input v-model="newCollKey" class="mono" placeholder="mydb.orders">
            </div>
            <div class="form-row">
              <label>TTL (s)</label>
              <input v-model.number="newCollTtl" type="number" min="1" style="max-width: 100px;">
            </div>
            <div class="form-row">
              <label>Enabled</label>
              <label class="toggle-row" style="padding-top: 0.4rem;">
                <span class="toggle">
                  <input v-model="newCollEnabled" type="checkbox">
                  <span class="toggle-slider" />
                </span>
                <span class="text-sm">{{ newCollEnabled ? 'Yes' : 'No' }}</span>
              </label>
            </div>
            <button class="btn btn-primary" type="button" :disabled="collBusy" @click="addCollection">
              Save policy
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

      <!-- Invalidate -->
      <div v-if="tab === 'invalidate'" class="card">
        <h3 class="card-title">Explicit cache invalidation</h3>
        <p class="card-desc">
          Clear cached entries for this tenant. Optionally scope by database, collection, and/or tags (Phase 3).
        </p>
        <div class="grid-2">
          <div class="form-row">
            <label>Database (optional)</label>
            <input v-model="invDb" class="mono" placeholder="mydb">
          </div>
          <div class="form-row">
            <label>Collection (optional)</label>
            <input v-model="invColl" class="mono" placeholder="orders">
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