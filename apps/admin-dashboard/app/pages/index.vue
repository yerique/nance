<script setup lang="ts">
import { Building2Icon, MailIcon, PlusIcon } from '@lucide/vue'
import { toast } from 'vue-sonner'
import type { OrganizationInvite, OrganizationSummary, PlatformSettings } from '~/types/accelerator'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Field, FieldDescription, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const api = useAcceleratorApi()
const auth = useAuth()

const orgs = ref<OrganizationSummary[]>([])
const invites = ref<OrganizationInvite[]>([])
const platform = ref<PlatformSettings | null>(null)
const loading = ref(true)
const error = ref('')
const showCreate = ref(false)
const creating = ref(false)
const createError = ref('')
const form = reactive({ id: '', name: '' })
const accepting = ref<string | null>(null)

const allowCreate = computed(() => platform.value?.allowOrgCreation !== false)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [o, inv, plat] = await Promise.all([
      api.listOrganizations(),
      api.listMyInvites(),
      api.getPlatformSettings().catch(() => ({ inviteOnly: false, allowOrgCreation: true, allowAdminBootstrap: true } as PlatformSettings)),
    ])
    orgs.value = o
    invites.value = inv
    platform.value = plat
  }
  catch (e) {
    error.value = api.apiErrorMessage(e)
  }
  finally {
    loading.value = false
  }
}

async function onCreate() {
  createError.value = ''
  if (!allowCreate.value) {
    createError.value = 'This instance is invite-only. Ask an organization admin to invite you.'
    return
  }
  if (!form.name.trim()) {
    createError.value = 'Organization name is required'
    return
  }
  creating.value = true
  try {
    const org = await api.createOrganization(form.name.trim(), form.id.trim() || undefined)
    showCreate.value = false
    form.id = ''
    form.name = ''
    toast.success('Organization created')
    await navigateTo(`/tenants/${encodeURIComponent(org.id)}`)
  }
  catch (e) {
    createError.value = api.apiErrorMessage(e)
  }
  finally {
    creating.value = false
  }
}

async function onAccept(id: string) {
  accepting.value = id
  try {
    const org = await api.acceptInvite(id)
    toast.success('Invite accepted')
    await navigateTo(`/tenants/${encodeURIComponent(org.id)}`)
  }
  catch (e) {
    error.value = api.apiErrorMessage(e)
  }
  finally {
    accepting.value = null
  }
}

onMounted(() => {
  auth.loadFromStorage()
  if (auth.isLoggedIn.value) load()
})

watch(() => auth.isLoggedIn.value, (v) => { if (v) load() })
</script>

<template>
  <div class="page-shell flex flex-col gap-6">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div class="flex flex-col gap-1.5">
        <p class="wire-label">Directory</p>
        <h1 class="text-2xl font-semibold tracking-tight">Organizations</h1>
        <p class="max-w-xl text-sm text-muted-foreground">
          Manage backends, proxy tokens, and cache TTL. App queries opt into caching with the
          <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">_cache</code>
          collection suffix (default TTL 60s).
        </p>
      </div>
      <Button v-if="allowCreate" @click="showCreate = true">
        <PlusIcon data-icon="inline-start" />
        New organization
      </Button>
    </div>

    <Alert v-if="platform?.inviteOnly">
      <MailIcon />
      <AlertTitle>Invite-only instance</AlertTitle>
      <AlertDescription>
        An operator enabled
        <code class="font-mono text-xs">NANCE_INVITE_ONLY</code>
        on this server. You can sign in, but you can only join organizations you are invited to —
        creating a new organization is disabled.
      </AlertDescription>
    </Alert>

    <Alert v-if="error" variant="destructive">
      <AlertTitle>Something went wrong</AlertTitle>
      <AlertDescription>{{ error }}</AlertDescription>
    </Alert>

    <Card v-if="invites.length" class="border-primary/30 bg-primary/5">
      <CardHeader class="pb-3">
        <p class="wire-label">Pending</p>
        <CardTitle class="text-base">Organization invites</CardTitle>
        <CardDescription>Accept to join with the role shown.</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-2">
        <div
          v-for="inv in invites"
          :key="inv.id"
          class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border/80 bg-card px-3 py-2.5"
        >
          <div class="flex flex-wrap items-center gap-2">
            <span class="font-medium">{{ inv.tenantName || inv.tenantId }}</span>
            <Badge :variant="roleBadgeVariant(inv.role)">{{ inv.role }}</Badge>
          </div>
          <Button size="sm" :disabled="accepting === inv.id" @click="onAccept(inv.id)">
            <Spinner v-if="accepting === inv.id" data-icon="inline-start" />
            {{ accepting === inv.id ? 'Accepting…' : 'Accept' }}
          </Button>
        </div>
      </CardContent>
    </Card>

    <template v-if="loading">
      <div class="flex flex-col gap-3">
        <Skeleton class="h-10 w-full" />
        <Skeleton class="h-10 w-full" />
        <Skeleton class="h-10 w-2/3" />
      </div>
    </template>

    <Empty v-else-if="!orgs.length" class="border border-dashed">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <Building2Icon />
        </EmptyMedia>
        <EmptyTitle>No organizations yet</EmptyTitle>
        <EmptyDescription v-if="platform?.inviteOnly">
          <template v-if="invites.length">
            Accept an invite above to get started.
          </template>
          <template v-else>
            This instance is invite-only. When an owner or admin invites
            <strong v-if="auth.user">{{ auth.user.email }}</strong><span v-else>your email</span>,
            the invitation will show up here.
          </template>
        </EmptyDescription>
        <EmptyDescription v-else>
          Create an organization to configure a MongoDB backend and issue proxy tokens.
        </EmptyDescription>
      </EmptyHeader>
      <EmptyContent v-if="allowCreate && !platform?.inviteOnly">
        <Button @click="showCreate = true">
          <PlusIcon data-icon="inline-start" />
          Create organization
        </Button>
      </EmptyContent>
    </Empty>

    <Card v-else class="overflow-hidden p-0">
      <Table>
        <TableHeader>
          <TableRow class="hover:bg-transparent">
            <TableHead>Name</TableHead>
            <TableHead>ID</TableHead>
            <TableHead>Your role</TableHead>
            <TableHead>Status</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow
            v-for="o in orgs"
            :key="o.id"
            class="cursor-pointer"
            @click="navigateTo(`/tenants/${encodeURIComponent(o.id)}`)"
          >
            <TableCell class="font-medium">
              <NuxtLink
                :to="`/tenants/${encodeURIComponent(o.id)}`"
                class="text-foreground hover:text-primary"
                @click.stop
              >
                {{ o.name }}
              </NuxtLink>
            </TableCell>
            <TableCell>
              <code class="font-mono text-xs text-muted-foreground">{{ o.id }}</code>
            </TableCell>
            <TableCell>
              <Badge :variant="roleBadgeVariant(o.role)">{{ o.role }}</Badge>
            </TableCell>
            <TableCell>
              <Badge :variant="statusBadgeVariant(o.status)">{{ o.status }}</Badge>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </Card>

    <Dialog v-model:open="showCreate">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create organization</DialogTitle>
          <DialogDescription>
            A tenant identity for backends, cache policy, and proxy tokens.
          </DialogDescription>
        </DialogHeader>

        <Alert v-if="createError" variant="destructive">
          <AlertTitle>Could not create</AlertTitle>
          <AlertDescription>{{ createError }}</AlertDescription>
        </Alert>

        <FieldGroup>
          <Field>
            <FieldLabel for="org-name">Name</FieldLabel>
            <Input id="org-name" v-model="form.name" type="text" placeholder="Acme Corp" required />
          </Field>
          <Field>
            <FieldLabel for="org-id">ID (optional)</FieldLabel>
            <Input id="org-id" v-model="form.id" type="text" placeholder="acme-corp" class="font-mono" />
            <FieldDescription>Stable slug used as tenant ID. Auto-generated if empty.</FieldDescription>
          </Field>
        </FieldGroup>

        <DialogFooter>
          <Button variant="outline" :disabled="creating" @click="showCreate = false">
            Cancel
          </Button>
          <Button :disabled="creating" @click="onCreate">
            <Spinner v-if="creating" data-icon="inline-start" />
            {{ creating ? 'Creating…' : 'Create' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
