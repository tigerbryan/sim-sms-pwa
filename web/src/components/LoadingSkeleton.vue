<script setup lang="ts">
defineProps<{ label: string }>()
</script>

<template>
  <section class="loading-skeleton" aria-live="polite" aria-busy="true">
    <span class="skeleton-line wide"></span>
    <span class="skeleton-line"></span>
    <span class="skeleton-line short"></span>
    <p>{{ label }}</p>
  </section>
</template>
