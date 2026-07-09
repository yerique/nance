<script setup lang="ts">
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

const api = useAcceleratorApi()
const auth = useAuth()

const passwordAuthEnabled = ref(false)
const loading = ref(false)
const error = ref('')
const currentPassword = ref('')
const password = ref('')
const confirm = ref('')

const hasPassword = computed(() => !!auth.user.value?.hasPassword)

onMounted(async () => {
  auth.loadFromStorage()
  try {
    const [plat, me] = await Promise.all([
      api.getPlatformSettings(),
      api.me().catch(() => null),
    ])
    passwordAuthEnabled.value = !!plat.passwordAuthEnabled
    if (me) {
      auth.setSession(auth.token.value!, me)
    }
    if (!passwordAuthEnabled.value) {
      await navigateTo('/')
    }
  }
  catch {
    await navigateTo('/')
  }
})

async function save() {
  error.value = ''
  if (password.value.length < 8) {
    error.value = 'Password must be at least 8 characters'
    return
  }
  if (password.value !== confirm.value) {
    error.value = 'Passwords do not match'
    return
  }
  if (hasPassword.value && !currentPassword.value) {
    error.value = 'Enter your current password'
    return
  }
  loading.value = true
  try {
    const user = await api.setPassword(
      password.value,
      hasPassword.value ? currentPassword.value : undefined,
    )
    if (auth.token.value) {
      auth.setSession(auth.token.value, user)
    }
    currentPassword.value = ''
    password.value = ''
    confirm.value = ''
    toast.success(hasPassword.value ? 'Password updated' : 'Password set — you can sign in with it next time')
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
  <div class="mx-auto flex w-full max-w-lg flex-col gap-6 px-4 py-8 sm:px-6">
    <div>
      <p class="wire-label">Account</p>
      <h1 class="text-2xl font-semibold tracking-tight">Password</h1>
      <p class="mt-1 text-sm text-muted-foreground">
        Optional password login for {{ auth.user?.email }}. You can always sign in with an email code.
      </p>
    </div>

    <Card>
      <CardHeader>
        <CardTitle class="text-base">
          {{ hasPassword ? 'Update password' : 'Set a password' }}
        </CardTitle>
        <CardDescription>
          <template v-if="hasPassword">
            Enter your current password, then choose a new one (min 8 characters).
          </template>
          <template v-else>
            Available only after your account exists. Min 8 characters.
          </template>
        </CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <Alert v-if="error" variant="destructive">
          <AlertTitle>Could not save</AlertTitle>
          <AlertDescription>{{ error }}</AlertDescription>
        </Alert>
        <form class="flex flex-col gap-4" @submit.prevent="save">
          <FieldGroup>
            <Field v-if="hasPassword">
              <FieldLabel for="current">Current password</FieldLabel>
              <Input
                id="current"
                v-model="currentPassword"
                type="password"
                autocomplete="current-password"
                :disabled="loading"
              />
            </Field>
            <Field>
              <FieldLabel for="password">{{ hasPassword ? 'New password' : 'Password' }}</FieldLabel>
              <Input
                id="password"
                v-model="password"
                type="password"
                autocomplete="new-password"
                required
                :disabled="loading"
              />
              <FieldDescription>At least 8 characters.</FieldDescription>
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
            </Field>
          </FieldGroup>
          <Button type="submit" :disabled="loading">
            <Spinner v-if="loading" data-icon="inline-start" />
            {{ loading ? 'Saving…' : (hasPassword ? 'Update password' : 'Set password') }}
          </Button>
        </form>
      </CardContent>
      <CardFooter class="text-xs text-muted-foreground">
        Forgot your password? Sign out and use “Forgot password?” on the login page.
      </CardFooter>
    </Card>
  </div>
</template>
