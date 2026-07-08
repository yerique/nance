<script setup lang="ts">
import { toast } from 'vue-sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'

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
    toast.success('Profile saved')
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
  <div class="auth-lattice flex min-h-svh items-center justify-center p-4 sm:p-8">
    <div class="flex w-full max-w-md flex-col gap-6">
      <div class="flex flex-col items-center gap-3 text-center">
        <img
          src="/nance-icon.svg"
          alt="Nance"
          width="48"
          height="48"
          class="size-12 object-contain drop-shadow-[0_0_24px_rgba(251,146,60,0.35)]"
        >
        <div class="flex flex-col gap-1">
          <p class="wire-label">First run</p>
          <h1 class="text-2xl font-semibold tracking-tight">Welcome aboard</h1>
          <p class="text-sm text-muted-foreground">
            What should teammates see when you act in an organization?
          </p>
        </div>
      </div>

      <Card class="border-border/80 shadow-lg shadow-black/20">
        <CardHeader class="border-b border-border/60 pb-4">
          <CardTitle class="text-base">Display name</CardTitle>
          <CardDescription v-if="auth.user">
            Signed in as <span class="font-medium text-foreground">{{ auth.user.email }}</span>
          </CardDescription>
        </CardHeader>
        <CardContent class="pt-5">
          <Alert v-if="error" variant="destructive" class="mb-4">
            <AlertTitle>Could not save</AlertTitle>
            <AlertDescription>{{ error }}</AlertDescription>
          </Alert>

          <form class="flex flex-col gap-4" @submit.prevent="save">
            <FieldGroup>
              <Field>
                <FieldLabel for="name">Your name</FieldLabel>
                <Input
                  id="name"
                  v-model="name"
                  type="text"
                  autocomplete="name"
                  placeholder="Ada Lovelace"
                  required
                  autofocus
                  :disabled="loading"
                />
              </Field>
            </FieldGroup>
            <Button type="submit" class="w-full" :disabled="loading">
              <Spinner v-if="loading" data-icon="inline-start" />
              {{ loading ? 'Saving…' : 'Continue' }}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
