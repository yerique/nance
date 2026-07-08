<script setup lang="ts">
import { Building2Icon, LogOutIcon } from '@lucide/vue'
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
  </div>
</template>
