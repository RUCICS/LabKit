<script setup lang="ts">
import { computed } from 'vue';
import type { QuotaSummary } from '../board/types';
import { formatQuotaSummary } from '../../lib/labs';

const props = defineProps<{
  quota?: QuotaSummary | null;
}>();

const summary = computed(() => formatQuotaSummary(props.quota));
const bonusRemaining = computed(() => props.quota?.bonus?.remaining ?? 0);
const dailyExhausted = computed(() => (props.quota?.left ?? 0) <= 0);
const showBonusHint = computed(() => dailyExhausted.value && bonusRemaining.value > 0);
</script>

<template>
  <p v-if="quota" class="quota-summary">
    <strong>{{ summary }}</strong>
    <span>{{ quota?.reset_hint }}</span>
    <span v-if="bonusRemaining > 0" class="quota-summary__bonus" data-testid="quota-bonus">
      · {{ bonusRemaining }} bonus left
    </span>
    <span v-if="showBonusHint" class="quota-summary__hint" data-testid="quota-bonus-hint">
      next submission will spend 1 bonus credit
    </span>
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

.quota-summary__bonus {
  color: var(--text-secondary);
}

.quota-summary__hint {
  flex-basis: 100%;
  color: var(--text-tertiary);
  text-transform: none;
  letter-spacing: 0;
  font-family: var(--font-sans, inherit);
  font-size: 0.78rem;
}
</style>
