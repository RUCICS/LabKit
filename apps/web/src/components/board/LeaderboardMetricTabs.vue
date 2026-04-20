<script setup lang="ts">
import type { LeaderboardMetric } from './types';
import { metricTone } from '../../lib/labs';

defineProps<{
  metrics: LeaderboardMetric[];
  selectedMetricId: string;
}>();

const emit = defineEmits<{
  select: [metricId: string];
}>();

function handleSelect(metricId: string) {
  emit('select', metricId);
}

function metricToneClass(metricId: string) {
  return `board-tabs__dot--${metricTone(metricId)}`;
}
</script>

<template>
  <div class="board-tabs" role="tablist" aria-label="Metrics">
    <button
      v-for="metric in metrics"
      :key="metric.id"
      type="button"
      class="board-tabs__button"
      role="tab"
      :aria-selected="metric.id === selectedMetricId"
      :class="{ 'board-tabs__button--selected': metric.id === selectedMetricId }"
      @click="handleSelect(metric.id)"
    >
      <span class="board-tabs__dot" :class="metricToneClass(metric.id)" />
      <span class="board-tabs__name">{{ metric.name }}</span>
    </button>
  </div>
</template>

<style scoped>
.board-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0;
  padding: 3px;
  border: 1px solid var(--border-default);
  border-radius: 8px;
  background: var(--bg-surface);
}

.board-tabs__button {
  display: inline-grid;
  grid-template-columns: auto 1fr;
  align-items: center;
  column-gap: 10px;
  min-width: 0;
  padding: 8px 20px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  text-align: left;
  transition: background 150ms ease, color 150ms ease, border-color 150ms ease;
}

.board-tabs__button:hover:not(.board-tabs__button--selected) {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.board-tabs__button:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

.board-tabs__button--selected {
  border-color: color-mix(in srgb, var(--accent) 20%, transparent);
  background: var(--accent-dim);
  color: var(--text-primary);
}

.board-tabs__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  transition: box-shadow 400ms ease;
}

.board-tabs__button--selected .board-tabs__dot {
  box-shadow: 0 0 6px 2px currentColor;
}

.board-tabs__dot--throughput {
  background: var(--track-throughput);
  color: var(--track-throughput);
}

.board-tabs__dot--latency {
  background: var(--track-latency);
  color: var(--track-latency);
}

.board-tabs__dot--fairness {
  background: var(--track-fairness);
  color: var(--track-fairness);
}

.board-tabs__name {
  font-family: var(--font-mono);
  font-size: 0.78rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
</style>
