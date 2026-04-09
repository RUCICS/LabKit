<script setup lang="ts">
import { computed } from 'vue';
import type { QuotaSummary } from '../board/types';
import { formatQuotaSummary } from '../../lib/labs';

const props = defineProps<{
  quota?: QuotaSummary | null;
}>();

const summary = computed(() => formatQuotaSummary(props.quota));
</script>

<template>
  <p v-if="quota" class="quota-summary">
    <strong>{{ summary }}</strong>
    <span>{{ quota?.reset_hint }}</span>
  </p>
</template>

<style scoped>
.quota-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 0;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.quota-summary strong {
  color: var(--text-primary);
  font-weight: 600;
}

.quota-summary span {
  color: var(--text-tertiary);
}
</style>
