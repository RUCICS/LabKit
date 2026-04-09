<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  value: string;
}>();

const tone = computed(() => {
  switch (props.value) {
    case 'scored':
      return 'scored';
    case 'build_failed':
      return 'build-failed';
    case 'rejected':
      return 'rejected';
    case 'error':
    case 'timeout':
      return 'error';
    case 'running':
      return 'running';
    default:
      return 'queued';
  }
});
</script>

<template>
  <span class="verdict-badge" :data-tone="tone">{{ value }}</span>
</template>

<style scoped>
.verdict-badge {
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  padding: 0 10px;
  border: 1px solid var(--border-default);
  border-radius: 6px;
  background: var(--bg-elevated);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.verdict-badge[data-tone='scored'] {
  color: var(--color-scored);
  border-color: rgba(52, 211, 153, 0.15);
  background: rgba(52, 211, 153, 0.08);
}

.verdict-badge[data-tone='build-failed'],
.verdict-badge[data-tone='error'] {
  color: var(--color-build-failed);
  border-color: rgba(248, 113, 113, 0.15);
  background: rgba(248, 113, 113, 0.08);
}

.verdict-badge[data-tone='rejected'] {
  color: var(--color-rejected);
  border-color: rgba(251, 146, 60, 0.15);
  background: rgba(251, 146, 60, 0.08);
}

.verdict-badge[data-tone='running'] {
  color: var(--track-latency);
  border-color: rgba(6, 182, 212, 0.15);
  background: rgba(6, 182, 212, 0.08);
}

.verdict-badge[data-tone='queued'] {
  color: var(--text-secondary);
}
</style>
