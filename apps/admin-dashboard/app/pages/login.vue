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
import { Field, FieldDescription, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'

definePageMeta({ layout: false })

const api = useAcceleratorApi()
const auth = useAuth()

const step = ref<'email' | 'code'>('email')
const email = ref('')
const code = ref('')
const loading = ref(false)
const error = ref('')
const inviteOnly = ref(false)

function needsOnboarding(name?: string | null) {
  return !name || !String(name).trim()
}

onMounted(async () => {
  auth.loadFromStorage()
  try {
    const plat = await api.getPlatformSettings()
    inviteOnly.value = !!plat.inviteOnly
  }
  catch { /* ignore */ }
  if (auth.isLoggedIn.value) {
    navigateTo(needsOnboarding(auth.user.value?.name) ? '/onboarding' : '/')
  }
})

async function sendCode() {
  error.value = ''
  if (!email.value.trim()) {
    error.value = 'Email is required'
    return
  }
  loading.value = true
  try {
    await api.requestCode(email.value.trim())
    step.value = 'code'
    toast.message('Check your email', {
      description: 'We sent a 6-digit code (also printed in control plane logs in dev).',
    })
  }
  catch (e) {
    error.value = api.apiErrorMessage(e)
  }
  finally {
    loading.value = false
  }
}

async function verify() {
  error.value = ''
  if (!code.value.trim()) {
    error.value = 'Enter the verification code'
    return
  }
  loading.value = true
  try {
    const res = await api.verifyCode(email.value.trim(), code.value.trim())
    auth.setSession(res.token, res.user)
    await navigateTo(needsOnboarding(res.user?.name) ? '/onboarding' : '/')
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
          <p class="wire-label">Nance accelerator</p>
          <h1 class="text-2xl font-semibold tracking-tight text-foreground">
            Sign in to the control plane
          </h1>
          <p class="text-sm text-muted-foreground">
            Passwordless email code — no password to store or rotate.
          </p>
        </div>
      </div>

      <Card class="border-border/80 shadow-lg shadow-black/20">
        <CardHeader class="border-b border-border/60 pb-4">
          <CardTitle class="text-base">
            {{ step === 'email' ? 'Your work email' : 'Enter verification code' }}
          </CardTitle>
          <CardDescription>
            <template v-if="step === 'email'">
              We'll send a one-time code to verify it's you.
            </template>
            <template v-else>
              Code sent to <span class="font-medium text-foreground">{{ email }}</span>
            </template>
          </CardDescription>
        </CardHeader>

        <CardContent class="pt-5">
          <Alert v-if="inviteOnly" class="mb-4">
            <MailIcon />
            <AlertTitle>Invite-only instance</AlertTitle>
            <AlertDescription>
              You can sign in, then accept an organization invite. Creating new organizations is disabled.
            </AlertDescription>
          </Alert>

          <Alert v-if="error" variant="destructive" class="mb-4">
            <AlertTitle>Could not continue</AlertTitle>
            <AlertDescription>{{ error }}</AlertDescription>
          </Alert>

          <form v-if="step === 'email'" class="flex flex-col gap-4" @submit.prevent="sendCode">
            <FieldGroup>
              <Field>
                <FieldLabel for="email">Email</FieldLabel>
                <Input
                  id="email"
                  v-model="email"
                  type="email"
                  autocomplete="email"
                  placeholder="you@company.com"
                  required
                  :disabled="loading"
                />
              </Field>
            </FieldGroup>
            <Button type="submit" class="w-full" :disabled="loading">
              <Spinner v-if="loading" data-icon="inline-start" />
              {{ loading ? 'Sending…' : 'Continue' }}
            </Button>
          </form>

          <form v-else class="flex flex-col gap-4" @submit.prevent="verify">
            <FieldGroup>
              <Field>
                <FieldLabel for="code">Verification code</FieldLabel>
                <Input
                  id="code"
                  v-model="code"
                  type="text"
                  inputmode="numeric"
                  autocomplete="one-time-code"
                  placeholder="123456"
                  class="font-mono tracking-widest"
                  required
                  :disabled="loading"
                />
                <FieldDescription>6-digit code from your inbox.</FieldDescription>
              </Field>
            </FieldGroup>
            <Button type="submit" class="w-full" :disabled="loading">
              <Spinner v-if="loading" data-icon="inline-start" />
              {{ loading ? 'Verifying…' : 'Sign in' }}
            </Button>
            <Button
              type="button"
              variant="ghost"
              class="w-full"
              :disabled="loading"
              @click="step = 'email'"
            >
              <ArrowLeftIcon data-icon="inline-start" />
              Use a different email
            </Button>
          </form>
        </CardContent>

        <CardFooter class="justify-center border-t border-border/60 pt-4">
          <p class="wire-label text-center">Proxy · policy · tokens</p>
        </CardFooter>
      </Card>
    </div>
  </div>
</template>
