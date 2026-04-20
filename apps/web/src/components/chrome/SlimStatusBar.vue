<script setup lang="ts">
defineProps<{
  label: string;
  value?: string;
  tone?: 'neutral' | 'good' | 'warn' | 'bad';
}>();
</script>

<template>
  <div class="slim-status-bar" :data-tone="tone ?? 'neutral'" role="status">
    <span class="slim-status-bar__label">{{ label }}</span>
    <span v-if="value || $slots.value" class="slim-status-bar__value">
      <slot name="value">
        {{ value }}
      </slot>
    </span>
    <span v-if="$slots.trailing" class="slim-status-bar__trailing">
      <slot name="trailing" />
    </span>
  </div>
</template>

<style scoped>
.slim-status-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 28px;
  padding: 6px 12px;
  border: 1px solid var(--border-default);
  border-radius: 8px;
  background: rgba(17, 26, 46, 0.6);
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.slim-status-bar__label {
  color: var(--text-tertiary);
}

.slim-status-bar__value {
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}

.slim-status-bar__trailing {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--text-tertiary);
}

.slim-status-bar[data-tone='good'] {
  border-color: rgba(52, 211, 153, 0.18);
}

.slim-status-bar[data-tone='warn'] {
  border-color: rgba(251, 146, 60, 0.18);
}

.slim-status-bar[data-tone='bad'] {
  border-color: rgba(248, 113, 113, 0.18);
}
</style>
