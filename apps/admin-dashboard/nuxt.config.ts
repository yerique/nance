// https://nuxt.com/docs/api/configuration/nuxt-config
import tailwindcss from '@tailwindcss/vite'

export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  css: ['~/assets/css/main.css'],
  modules: ['shadcn-nuxt'],
  shadcn: {
    prefix: '',
    componentDir: '@/components/ui',
  },
  vite: {
    plugins: [
      tailwindcss(),
    ],
  },
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
      htmlAttrs: {
        class: 'dark',
        style: 'color-scheme: dark',
      },
      title: 'Nance Admin',
      meta: [
        { name: 'description', content: 'Nance accelerator control plane admin dashboard' },
        { name: 'color-scheme', content: 'dark' },
        { name: 'theme-color', content: '#0b1220' },
      ],
    },
  },
})
