<script setup lang="ts">
import { Toaster } from '@/components/ui/sonner'
import 'vue-sonner/style.css'
</script>

<template>
  <div class="min-h-svh bg-background text-foreground">
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>
    <Toaster position="top-right" :close-button="true" theme="dark" rich-colors />
  </div>
</template>
