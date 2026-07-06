export function formatDate(value?: string | null): string {
  if (!value) return '—'
  try {
    return new Date(value).toLocaleString(undefined, {
      dateStyle: 'medium',
      timeStyle: 'short',
    })
  }
  catch {
    return value
  }
}

/** Map status strings to shadcn Badge variants. */
export function statusBadgeVariant(status?: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  const s = (status || '').toLowerCase()
  if (s === 'active' || s === 'ok') return 'default'
  if (s === 'disabled' || s === 'revoked') return 'destructive'
  if (s === 'pending') return 'outline'
  return 'secondary'
}

export function roleBadgeVariant(role?: string): 'default' | 'secondary' | 'outline' {
  const r = (role || '').toLowerCase()
  if (r === 'owner') return 'default'
  if (r === 'admin') return 'outline'
  return 'secondary'
}
