<script setup lang="ts">
const auth = useAuth()
const api = useAcceleratorApi()
const route = useRoute()

function needsOnboarding() {
  return auth.isLoggedIn.value && !auth.user.value?.name?.trim()
}

onMounted(() => {
  auth.loadFromStorage()
})

watch([() => auth.ready.value, () => auth.isLoggedIn.value, () => auth.user.value?.name, () => route.path], () => {
  if (!auth.ready.value) return
  if (!auth.isLoggedIn.value && route.path !== '/login') {
    navigateTo('/login')
    return
  }
  if (auth.isLoggedIn.value && needsOnboarding() && route.path !== '/onboarding') {
    navigateTo('/onboarding')
  }
}, { immediate: true })

async function onLogout() {
  try {
    await api.logout()
  }
  catch { /* ignore */ }
  auth.clearSession()
  await navigateTo('/login')
}
</script>

<template>
  <div class="app-shell">
    <header class="topbar">
      <NuxtLink to="/" class="brand">
        <span class="logo-mark">N</span>
        <span>Nance Admin</span>
      </NuxtLink>
      <nav class="nav">
        <NuxtLink to="/">Organizations</NuxtLink>
      </nav>
      <div v-if="auth.user" class="user-menu">
        <span class="user-email">{{ auth.user.name || auth.user.email }}</span>
        <button class="btn btn-ghost btn-sm" type="button" @click="onLogout">Sign out</button>
      </div>
    </header>
    <main class="main">
      <slot />
    </main>
  </div>
</template>

<style scoped>
.app-shell { min-height: 100vh; display: flex; flex-direction: column; }
.topbar {
  display: flex;
  align-items: center;
  gap: 1.5rem;
  padding: 0.75rem 1.5rem;
  border-bottom: 1px solid var(--border, #2a2f3a);
  background: var(--surface, #12151c);
}
.brand {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  font-weight: 600;
  text-decoration: none;
  color: inherit;
}
.logo-mark {
  width: 1.75rem;
  height: 1.75rem;
  border-radius: 0.35rem;
  background: var(--accent, #5b8def);
  color: #fff;
  display: grid;
  place-items: center;
  font-size: 0.85rem;
  font-weight: 700;
}
.nav { display: flex; gap: 1rem; flex: 1; }
.nav a { color: inherit; opacity: 0.8; text-decoration: none; }
.nav a.router-link-active { opacity: 1; font-weight: 600; }
.user-menu { display: flex; align-items: center; gap: 0.75rem; }
.user-email { font-size: 0.85rem; opacity: 0.75; }
.main { flex: 1; padding: 1.5rem; max-width: 1100px; width: 100%; margin: 0 auto; }
.btn-sm { padding: 0.35rem 0.65rem; font-size: 0.8rem; }
</style>
