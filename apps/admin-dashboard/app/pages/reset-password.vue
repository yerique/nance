<script setup lang="ts">
import { ArrowLeftIcon } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Field, FieldDescription, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'

definePageMeta({ layout: false })

const route = useRoute()
const api = useAcceleratorApi()
const password = ref('')
const confirm = ref('')
const loading = ref(false)
const error = ref('')
const enabled = ref(false)
const token = computed(() => String(route.query.token || ''))

onMounted(async () => {
  try {
    const plat = await api.getPlatformSettings()
    enabled.value = !!plat.passwordAuthEnabled
    if (!enabled.value) {
      await navigateTo('/login')
    }
  }
  catch {
    await navigateTo('/login')
  }
})

async function submit() {
  error.value = ''
  if (!token.value) {
    error.value = 'Missing reset token. Open the link from your email.'
    return
  }
  if (password.value.length < 8) {
    error.value = 'Password must be at least 8 characters'
    return
  }
  if (password.value !== confirm.value) {
    error.value = 'Passwords do not match'
    return
  }
  loading.value = true
  try {
    await api.resetPassword(token.value, password.value)
    toast.success('Password updated — sign in with your new password')
    await navigateTo('/login')
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
        <img src="/nance-icon.svg" alt="Nance" width="48" height="48" class="size-12 object-contain">
        <h1 class="text-2xl font-semibold tracking-tight">Choose a new password</h1>
      </div>

      <Card class="border-border/80 shadow-lg shadow-black/20">
        <CardHeader class="border-b border-border/60 pb-4">
          <CardTitle class="text-base">New password</CardTitle>
          <CardDescription>At least 8 characters.</CardDescription>
        </CardHeader>
        <CardContent class="pt-5">
          <Alert v-if="error" variant="destructive" class="mb-4">
            <AlertTitle>Could not reset</AlertTitle>
            <AlertDescription>{{ error }}</AlertDescription>
          </Alert>
          <form class="flex flex-col gap-4" @submit.prevent="submit">
            <FieldGroup>
              <Field>
                <FieldLabel for="password">Password</FieldLabel>
                <Input
                  id="password"
                  v-model="password"
                  type="password"
                  autocomplete="new-password"
                  required
                  :disabled="loading"
                />
              </Field>
              <Field>
                <FieldLabel for="confirm">Confirm</FieldLabel>
                <Input
                  id="confirm"
                  v-model="confirm"
                  type="password"
                  autocomplete="new-password"
                  required
                  :disabled="loading"
                />
                <FieldDescription>Must match the password above.</FieldDescription>
              </Field>
            </FieldGroup>
            <Button type="submit" class="w-full" :disabled="loading || !token">
              <Spinner v-if="loading" data-icon="inline-start" />
              {{ loading ? 'Saving…' : 'Update password' }}
            </Button>
          </form>
        </CardContent>
        <CardFooter class="justify-center border-t border-border/60 pt-4">
          <NuxtLink to="/login" class="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
            <ArrowLeftIcon class="size-3.5" />
            Back to sign in
          </NuxtLink>
        </CardFooter>
      </Card>
    </div>
  </div>
</template>
