<script setup lang="ts">
import { Building2Icon, KeyRoundIcon, LogOutIcon } from '@lucide/vue'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Separator } from '@/components/ui/separator'

const auth = useAuth()
const api = useAcceleratorApi()
const route = useRoute()
const passwordAuthEnabled = ref(false)

function needsOnboarding() {
  return auth.isLoggedIn.value && !auth.user.value?.name?.trim()
}

onMounted(async () => {
  auth.loadFromStorage()
  try {
    const plat = await api.getPlatformSettings()
    passwordAuthEnabled.value = !!plat.passwordAuthEnabled
  }
  catch { /* optional */ }
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

const initials = computed(() => {
  const name = auth.user.value?.name?.trim()
  const email = auth.user.value?.email || ''
  if (name) {
    return name.split(/\s+/).map(p => p[0]).slice(0, 2).join('').toUpperCase()
  }
  return (email[0] || 'U').toUpperCase()
})
</script>

<template>
  <div class="flex min-h-svh flex-col">
    <header class="sticky top-0 z-40 border-b border-border/80 bg-background/85 backdrop-blur-md">
      <div class="mx-auto flex h-14 w-full max-w-6xl items-center gap-4 px-4 sm:px-6 lg:px-8">
        <NuxtLink
          to="/"
          class="flex items-center gap-2.5 text-foreground no-underline transition-opacity hover:opacity-90"
        >
          <img
            src="/nance-icon.svg"
            alt=""
            width="28"
            height="28"
            class="size-7 shrink-0 object-contain"
          >
          <span class="flex flex-col leading-none">
            <span class="text-sm font-semibold tracking-tight">Nance</span>
            <span class="wire-label mt-0.5 text-[0.6rem] leading-none">Control plane</span>
          </span>
        </NuxtLink>

        <Separator orientation="vertical" class="hidden h-6 sm:block" />

        <nav class="hidden items-center gap-1 sm:flex">
          <Button
            variant="ghost"
            size="sm"
            as-child
            :class="route.path === '/' || route.path.startsWith('/tenants') ? 'bg-muted text-foreground' : ''"
          >
            <NuxtLink to="/">
              <Building2Icon data-icon="inline-start" />
              Organizations
            </NuxtLink>
          </Button>
        </nav>

        <div class="ml-auto flex items-center gap-2">
          <DropdownMenu v-if="auth.user">
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="sm" class="gap-2 pl-1.5">
                <Avatar class="size-6">
                  <AvatarFallback class="bg-primary/15 text-[0.65rem] font-semibold text-primary">
                    {{ initials }}
                  </AvatarFallback>
                </Avatar>
                <span class="hidden max-w-40 truncate text-sm sm:inline">
                  {{ auth.user.name || auth.user.email }}
                </span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-56">
              <DropdownMenuGroup>
                <DropdownMenuLabel class="font-normal">
                  <div class="flex flex-col gap-0.5">
                    <span class="text-sm font-medium">{{ auth.user.name || 'Account' }}</span>
                    <span class="truncate text-xs text-muted-foreground">{{ auth.user.email }}</span>
                  </div>
                </DropdownMenuLabel>
              </DropdownMenuGroup>
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                <DropdownMenuItem v-if="passwordAuthEnabled" as-child>
                  <NuxtLink to="/account">
                    <KeyRoundIcon />
                    Password
                  </NuxtLink>
                </DropdownMenuItem>
                <DropdownMenuItem @click="onLogout">
                  <LogOutIcon />
                  Sign out
                </DropdownMenuItem>
              </DropdownMenuGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </header>

    <main class="flex-1">
      <slot />
    </main>

    <footer class="border-t border-border/60 py-4">
      <div class="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-4 sm:px-6 lg:px-8">
        <p class="text-xs text-muted-foreground">
          Nance — open-source MongoDB accelerator
        </p>
        <a
          href="https://github.com/taeven/nance"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
        >
          <svg class="size-3.5 shrink-0" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
            <path d="M12 0C5.37 0 0 5.37 0 12c0 5.3 3.438 9.8 8.205 11.387.6.113.82-.26.82-.577 0-.285-.01-1.04-.016-2.04-3.338.726-4.042-1.61-4.042-1.61-.546-1.387-1.333-1.757-1.333-1.757-1.09-.745.083-.73.083-.73 1.205.085 1.84 1.237 1.84 1.237 1.07 1.834 2.807 1.304 3.492.997.108-.775.418-1.305.76-1.605-2.665-.303-5.467-1.334-5.467-5.933 0-1.31.468-2.382 1.236-3.222-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.3 1.23a11.5 11.5 0 0 1 3.003-.404c1.02.005 2.047.138 3.003.404 2.29-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.12 3.176.77.84 1.235 1.912 1.235 3.222 0 4.61-2.807 5.625-5.48 5.922.43.372.823 1.103.823 2.222 0 1.606-.015 2.898-.015 3.293 0 .32.216.694.825.576C20.565 21.796 24 17.297 24 12 24 5.37 18.63 0 12 0z" />
          </svg>
          GitHub
        </a>
      </div>
    </footer>
  </div>
</template>
