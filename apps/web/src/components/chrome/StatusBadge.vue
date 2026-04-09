<script setup lang="ts">
defineProps<{
  label: string;
  tone: 'open' | 'upcoming' | 'closed';
}>();
</script>

<template>
  <span class="status-badge" :data-tone="tone">
    <span class="status-badge__dot" aria-hidden="true" />
    {{ label }}
  </span>
</template>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 5px 10px;
  border: 1px solid var(--border-default);
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.status-badge[data-tone='open'] {
  color: var(--color-open);
  border-color: rgba(52, 211, 153, 0.15);
  background: rgba(52, 211, 153, 0.08);
}

.status-badge[data-tone='upcoming'],
.status-badge[data-tone='closed'] {
  background: var(--bg-elevated);
}

.status-badge[data-tone='upcoming'] {
  color: var(--text-secondary);
}

.status-badge[data-tone='closed'] {
  color: var(--text-tertiary);
}

.status-badge__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.status-badge[data-tone='open'] .status-badge__dot {
  animation: pulse-open 2s ease-in-out infinite;
}
</style>
