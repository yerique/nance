<script setup lang="ts">
const route = useRoute()
const api = useAcceleratorApi()

const health = ref<{ ok: boolean, accelerator: string } | null>(null)

onMounted(async () => {
  try {
    health.value = await api.checkHealth()
  }
  catch {
    health.value = { ok: false, accelerator: 'unknown' }
  }
})

function isActive(path: string) {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="sidebar-brand">
        <h1>Nance</h1>
        <p>Accelerator Admin</p>
      </div>
      <nav class="sidebar-nav">
        <NuxtLink
          to="/"
          class="nav-item"
          :class="{ active: isActive('/') && route.path === '/' }"
        >
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.75">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6A2.25 2.25 0 016 3.75h2.25A2.25 2.25 0 0110.5 6v2.25a2.25 2.25 0 01-2.25 2.25H6a2.25 2.25 0 01-2.25-2.25V6zM3.75 15.75A2.25 2.25 0 016 13.5h2.25a2.25 2.25 0 012.25 2.25V18a2.25 2.25 0 01-2.25 2.25H6A2.25 2.25 0 013.75 18v-2.25zM13.5 6a2.25 2.25 0 012.25-2.25H18A2.25 2.25 0 0120.25 6v2.25A2.25 2.25 0 0118 10.5h-2.25a2.25 2.25 0 01-2.25-2.25V6zM13.5 15.75a2.25 2.25 0 012.25-2.25H18a2.25 2.25 0 012.25 2.25V18A2.25 2.25 0 0118 20.25h-2.25A2.25 2.25 0 0113.5 18v-2.25z" />
          </svg>
          <span>Tenants</span>
        </NuxtLink>
      </nav>
      <div class="sidebar-footer">
        <div v-if="health" :class="health.ok ? 'text-success' : 'text-danger'">
          {{ health.ok ? '● Control plane OK' : '● Control plane unreachable' }}
        </div>
        <div v-if="health" class="text-dim mt-1" style="word-break: break-all;">
          {{ health.accelerator }}
        </div>
      </div>
    </aside>
    <main class="main">
      <slot />
    </main>
  </div>
</template>