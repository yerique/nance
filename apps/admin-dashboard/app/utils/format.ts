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

export function statusBadgeClass(status?: string): string {
  const s = (status || '').toLowerCase()
  if (s === 'active' || s === 'ok') return 'badge badge-success'
  if (s === 'disabled' || s === 'revoked') return 'badge badge-danger'
  if (s === 'pending') return 'badge badge-warning'
  return 'badge badge-muted'
}