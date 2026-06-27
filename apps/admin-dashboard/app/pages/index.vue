<script setup lang="ts">
import type { Tenant } from '~/types/accelerator'

const api = useAcceleratorApi()

const tenants = ref<Tenant[]>([])
const loading = ref(true)
const error = ref('')
const showCreate = ref(false)
const creating = ref(false)
const createError = ref('')
const form = reactive({ id: '', name: '' })

async function load() {
  loading.value = true
  error.value = ''
  try {
    tenants.value = await api.listTenants()
  }
  catch (e) {
    error.value = api.apiErrorMessage(e)
  }
  finally {
    loading.value = false
  }
}

async function onCreate() {
  createError.value = ''
  if (!form.id.trim() || !form.name.trim()) {
    createError.value = 'ID and name are required'
    return
  }
  creating.value = true
  try {
    const t = await api.createTenant(form.id.trim(), form.name.trim())
    showCreate.value = false
    form.id = ''
    form.name = ''
    await navigateTo(`/tenants/${encodeURIComponent(t.id)}`)
  }
  catch (e) {
    createError.value = api.apiErrorMessage(e)
  }
  finally {
    creating.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2>Tenants</h2>
        <p class="subtitle">Manage accelerator tenants, backends, cache policies, and access tokens.</p>
      </div>
      <button class="btn btn-primary" type="button" @click="showCreate = true">
        + New tenant
      </button>
    </div>

    <div v-if="error" class="alert alert-error">{{ error }}</div>

    <div v-if="loading" class="loading">Loading tenants…</div>

    <div v-else-if="!tenants.length" class="card empty-state">
      <p><strong>No tenants yet</strong></p>
      <p>Create a tenant to configure a MongoDB backend and issue proxy tokens.</p>
      <button class="btn btn-primary mt-2" type="button" @click="showCreate = true">
        Create first tenant
      </button>
    </div>

    <div v-else class="table-wrap">
      <table class="data-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>Name</th>
            <th>Status</th>
            <th>Created</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr v-for="t in tenants" :key="t.id">
            <td>
              <NuxtLink class="row-link mono" :to="`/tenants/${encodeURIComponent(t.id)}`">
                {{ t.id }}
              </NuxtLink>
            </td>
            <td>{{ t.name }}</td>
            <td>
              <span :class="statusBadgeClass(t.status)">{{ t.status || 'unknown' }}</span>
            </td>
            <td class="text-muted text-sm">{{ formatDate(t.created_at) }}</td>
            <td>
              <NuxtLink class="btn btn-ghost btn-sm" :to="`/tenants/${encodeURIComponent(t.id)}`">
                Manage →
              </NuxtLink>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Create modal -->
    <div v-if="showCreate" class="modal-backdrop" @click.self="showCreate = false">
      <div class="modal">
        <h3>Create tenant</h3>
        <div v-if="createError" class="alert alert-error">{{ createError }}</div>
        <div class="form-row">
          <label for="tenant-id">Tenant ID</label>
          <input
            id="tenant-id"
            v-model="form.id"
            class="mono"
            placeholder="e.g. demo or proj_abc123"
            autocomplete="off"
          >
          <span class="hint">Stable identifier used in proxy auth username and APIs.</span>
        </div>
        <div class="form-row">
          <label for="tenant-name">Display name</label>
          <input
            id="tenant-name"
            v-model="form.name"
            placeholder="My Project"
            autocomplete="off"
          >
        </div>
        <div class="form-actions">
          <button class="btn btn-primary" type="button" :disabled="creating" @click="onCreate">
            {{ creating ? 'Creating…' : 'Create tenant' }}
          </button>
          <button class="btn btn-secondary" type="button" :disabled="creating" @click="showCreate = false">
            Cancel
          </button>
        </div>
      </div>
    </div>
  </div>
</template>