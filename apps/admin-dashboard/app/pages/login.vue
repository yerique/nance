<script setup lang="ts">
import { ArrowLeftIcon, KeyRoundIcon, MailIcon } from '@lucide/vue'
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

const mode = ref<'code' | 'password'>('code')
const step = ref<'email' | 'code'>('email')
const email = ref('')
const code = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')
const inviteOnly = ref(false)
const passwordAuthEnabled = ref(false)

function needsOnboarding(name?: string | null) {
  return !name || !String(name).trim()
}

onMounted(async () => {
  auth.loadFromStorage()
  try {
    const plat = await api.getPlatformSettings()
    inviteOnly.value = !!plat.inviteOnly
    passwordAuthEnabled.value = !!plat.passwordAuthEnabled
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
      description: 'We sent a 6-digit verification code. It expires in 10 minutes.',
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

async function loginWithPassword() {
  error.value = ''
  if (!email.value.trim() || !password.value) {
    error.value = 'Email and password are required'
    return
  }
  loading.value = true
  try {
    const res = await api.loginPassword(email.value.trim(), password.value)
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

function switchMode(next: 'code' | 'password') {
  mode.value = next
  step.value = 'email'
  error.value = ''
  code.value = ''
  password.value = ''
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
            <template v-if="passwordAuthEnabled && mode === 'password'">
              Use the password you set after creating your account.
            </template>
            <template v-else>
              Passwordless email code — no password required.
            </template>
          </p>
        </div>
      </div>

      <Card class="border-border/80 shadow-lg shadow-black/20">
        <CardHeader class="border-b border-border/60 pb-4">
          <div v-if="passwordAuthEnabled" class="mb-3 flex gap-1 rounded-lg border border-border/60 bg-muted/30 p-1">
            <Button
              type="button"
              size="sm"
              class="flex-1"
              :variant="mode === 'code' ? 'secondary' : 'ghost'"
              @click="switchMode('code')"
            >
              <MailIcon data-icon="inline-start" />
              Email code
            </Button>
            <Button
              type="button"
              size="sm"
              class="flex-1"
              :variant="mode === 'password' ? 'secondary' : 'ghost'"
              @click="switchMode('password')"
            >
              <KeyRoundIcon data-icon="inline-start" />
              Password
            </Button>
          </div>
          <CardTitle class="text-base">
            <template v-if="mode === 'password'">Sign in with password</template>
            <template v-else-if="step === 'email'">Your work email</template>
            <template v-else>Enter verification code</template>
          </CardTitle>
          <CardDescription>
            <template v-if="mode === 'password'">
              Only works after you set a password from your account menu.
            </template>
            <template v-else-if="step === 'email'">
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

          <!-- Password login -->
          <form
            v-if="passwordAuthEnabled && mode === 'password'"
            class="flex flex-col gap-4"
            @submit.prevent="loginWithPassword"
          >
            <FieldGroup>
              <Field>
                <FieldLabel for="pw-email">Email</FieldLabel>
                <Input
                  id="pw-email"
                  v-model="email"
                  type="email"
                  autocomplete="email"
                  placeholder="you@company.com"
                  required
                  :disabled="loading"
                />
              </Field>
              <Field>
                <FieldLabel for="pw-password">Password</FieldLabel>
                <Input
                  id="pw-password"
                  v-model="password"
                  type="password"
                  autocomplete="current-password"
                  required
                  :disabled="loading"
                />
              </Field>
            </FieldGroup>
            <Button type="submit" class="w-full" :disabled="loading">
              <Spinner v-if="loading" data-icon="inline-start" />
              {{ loading ? 'Signing in…' : 'Sign in' }}
            </Button>
            <NuxtLink
              to="/forgot-password"
              class="text-center text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
            >
              Forgot password?
            </NuxtLink>
          </form>

          <!-- Email code -->
          <template v-else>
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
          </template>
        </CardContent>

        <CardFooter class="flex flex-col items-center gap-3 border-t border-border/60 pt-4">
          <p class="wire-label text-center">Proxy · policy · tokens</p>
          <a
            href="https://github.com/taeven/nance"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-2 rounded-full border border-border/70 bg-muted/40 px-3 py-1.5 text-xs font-medium text-muted-foreground transition-colors hover:border-border hover:bg-muted hover:text-foreground"
          >
            <svg class="size-3.5 shrink-0" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.3 3.438 9.8 8.205 11.387.6.113.82-.26.82-.577 0-.285-.01-1.04-.016-2.04-3.338.726-4.042-1.61-4.042-1.61-.546-1.387-1.333-1.757-1.333-1.757-1.09-.745.083-.73.083-.73 1.205.085 1.84 1.237 1.84 1.237 1.07 1.834 2.807 1.304 3.492.997.108-.775.418-1.305.76-1.605-2.665-.303-5.467-1.334-5.467-5.933 0-1.31.468-2.382 1.236-3.222-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.3 1.23a11.5 11.5 0 0 1 3.003-.404c1.02.005 2.047.138 3.003.404 2.29-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.12 3.176.77.84 1.235 1.912 1.235 3.222 0 4.61-2.807 5.625-5.48 5.922.43.372.823 1.103.823 2.222 0 1.606-.015 2.898-.015 3.293 0 .32.216.694.825.576C20.565 21.796 24 17.297 24 12 24 5.37 18.63 0 12 0z" />
            </svg>
            Open source on GitHub
          </a>
        </CardFooter>
      </Card>
    </div>
  </div>
</template>
