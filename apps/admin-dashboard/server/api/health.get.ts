export default defineEventHandler(async (event) => {
  const { baseUrl } = getAcceleratorConfig(event)
  try {
    const text = await $fetch<string>(`${baseUrl}/healthz`, { responseType: 'text' })
    return { ok: true, accelerator: baseUrl, health: text }
  }
  catch {
    return { ok: false, accelerator: baseUrl, health: null }
  }
})