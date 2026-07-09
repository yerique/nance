<script setup lang="ts">
import { ArrowLeftIcon, MailIcon } from '@lucide/vue'
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
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'

definePageMeta({ layout: false })

const api = useAcceleratorApi()
const email = ref('')
const loading = ref(false)
const error = ref('')
const sent = ref(false)
const enabled = ref(false)

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
  if (!email.value.trim()) {
    error.value = 'Email is required'
    return
  }
  loading.value = true
  try {
    await api.forgotPassword(email.value.trim())
    sent.value = true
    toast.message('Check your email', {
      description: 'If an account with a password exists, we sent a reset link.',
    })
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
        <h1 class="text-2xl font-semibold tracking-tight">Forgot password</h1>
        <p class="text-sm text-muted-foreground">
          We’ll email a reset link if this address has a password set.
        </p>
      </div>

      <Card class="border-border/80 shadow-lg shadow-black/20">
        <CardHeader class="border-b border-border/60 pb-4">
          <CardTitle class="text-base">Reset link</CardTitle>
          <CardDescription>Link expires in 60 minutes.</CardDescription>
        </CardHeader>
        <CardContent class="pt-5">
          <Alert v-if="error" variant="destructive" class="mb-4">
            <AlertTitle>Could not continue</AlertTitle>
            <AlertDescription>{{ error }}</AlertDescription>
          </Alert>
          <Alert v-if="sent" class="mb-4">
            <MailIcon />
            <AlertTitle>Request received</AlertTitle>
            <AlertDescription>
              If an account with a password exists for that email, a reset link is on the way.
            </AlertDescription>
          </Alert>
          <form v-if="!sent" class="flex flex-col gap-4" @submit.prevent="submit">
            <FieldGroup>
              <Field>
                <FieldLabel for="email">Email</FieldLabel>
                <Input
                  id="email"
                  v-model="email"
                  type="email"
                  autocomplete="email"
                  required
                  :disabled="loading"
                />
              </Field>
            </FieldGroup>
            <Button type="submit" class="w-full" :disabled="loading">
              <Spinner v-if="loading" data-icon="inline-start" />
              {{ loading ? 'Sending…' : 'Send reset link' }}
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
