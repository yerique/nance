<script setup lang="ts">
definePageMeta({ layout: false })

const api = useAcceleratorApi()
const auth = useAuth()

const name = ref('')
const loading = ref(false)
const error = ref('')

onMounted(() => {
  auth.loadFromStorage()
  if (!auth.isLoggedIn.value) {
    navigateTo('/login')
    return
  }
  // Already has a name — skip onboarding
  if (auth.user.value?.name?.trim()) {
    navigateTo('/')
  }
})

async function save() {
  error.value = ''
  if (!name.value.trim()) {
    error.value = 'Please enter your name'
    return
  }
  loading.value = true
  try {
    const user = await api.updateProfile(name.value.trim())
    auth.setSession(auth.token.value!, user)
    await navigateTo('/')
  }
  catch (e) {
    error.value = api.apiErrorMessage(e)
  }
  finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card card">
      <div class="login-brand">
        <span class="logo-mark">N</span>
        <div>
          <h1>Welcome</h1>
          <p class="subtitle">What should we call you?</p>
        </div>
      </div>

      <p v-if="auth.user" class="muted">Signed in as <strong>{{ auth.user.email }}</strong></p>
      <div v-if="error" class="alert alert-error">{{ error }}</div>

      <form class="stack" @submit.prevent="save">
        <label class="field">
          <span>Your name</span>
          <input
            v-model="name"
            type="text"
            autocomplete="name"
            placeholder="Ada Lovelace"
            required
            autofocus
          >
        </label>
        <button class="btn btn-primary" type="submit" :disabled="loading">
          {{ loading ? 'Saving…' : 'Continue' }}
        </button>
      </form>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 2rem;
  background: var(--bg, #0f1115);
}
.login-card {
  width: min(420px, 100%);
  padding: 2rem;
}
.login-brand {
  display: flex;
  gap: 1rem;
  align-items: center;
  margin-bottom: 1.5rem;
}
.logo-mark {
  width: 2.5rem;
  height: 2.5rem;
  border-radius: 0.5rem;
  background: var(--accent, #5b8def);
  color: #fff;
  display: grid;
  place-items: center;
  font-weight: 700;
}
.stack { display: flex; flex-direction: column; gap: 1rem; }
.field { display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.875rem; }
.field input {
  padding: 0.6rem 0.75rem;
  border-radius: 0.4rem;
  border: 1px solid var(--border, #2a2f3a);
  background: var(--surface-2, #161a22);
  color: inherit;
}
.muted { font-size: 0.9rem; opacity: 0.85; margin-bottom: 1rem; }
</style>
