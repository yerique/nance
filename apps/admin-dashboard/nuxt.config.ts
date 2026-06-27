// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  css: ['~/assets/css/main.css'],
  runtimeConfig: {
    // Server-only: accelerator control plane
    acceleratorBaseUrl: process.env.NANCE_ACCELERATOR_URL || 'http://localhost:8080',
    acceleratorAdminToken: process.env.NANCE_ADMIN_TOKEN || '',
    public: {
      appName: 'Nance Admin',
    },
  },
  app: {
    head: {
      title: 'Nance Admin',
      meta: [
        { name: 'description', content: 'Nance accelerator control plane admin dashboard' },
      ],
      link: [
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=IBM+Plex+Sans:wght@400;500;600;700&family=IBM+Plex+Mono:wght@400;500&display=swap',
        },
      ],
    },
  },
})
