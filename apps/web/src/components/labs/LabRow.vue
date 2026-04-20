<script setup lang="ts">
import { computed } from 'vue';
import StatusBadge from '../chrome/StatusBadge.vue';
import RowLink from '../chrome/RowLink.vue';
import type { PublicLab } from '../board/types';
import { getLabPhase, labPhaseLabel, metricTone } from '../../lib/labs';

const props = defineProps<{
  lab: PublicLab;
  to: string;
}>();

const phase = computed(() => getLabPhase(props.lab.manifest?.schedule));

const closeDate = computed(() => {
  const close = Date.parse(props.lab.manifest?.schedule?.close ?? '');
  if (Number.isNaN(close)) return 'TBD';
  return new Intl.DateTimeFormat('en-US', { month: '2-digit', day: '2-digit' }).format(close);
});

const metrics = computed(() => props.lab.manifest?.metrics ?? []);
const visibleMetrics = computed(() => metrics.value.slice(0, 3));
const overflowCount = computed(() => Math.max(0, metrics.value.length - visibleMetrics.value.length));

function metricDot(index: number) {
  return `lab-row__metric-dot--${metricTone(index)}`;
}
</script>

<template>
  <RowLink class="lab-row" :to="to" :aria-label="`Open ${lab.name}`">
    <span class="lab-row__name">{{ lab.name }}</span>

    <template #subtitle>
      <span class="lab-row__subtitle">
        <span class="lab-row__id">{{ lab.id }}</span>
        <span class="lab-row__sep" aria-hidden="true">·</span>
        <span class="lab-row__close">closes {{ closeDate }}</span>
      </span>
    </template>

    <template #meta>
      <span class="lab-row__meta">
        <StatusBadge :label="labPhaseLabel(phase)" :tone="phase" />
        <span class="lab-row__metrics">
          <span
            v-for="(metric, metricIndex) in visibleMetrics"
            :key="metric.id"
            class="lab-row__metric"
          >
            <span class="lab-row__metric-dot" :class="metricDot(metricIndex)" aria-hidden="true" />
            {{ metric.name }}
          </span>
          <span v-if="overflowCount" class="lab-row__metric lab-row__metric--overflow">
            +{{ overflowCount }}
          </span>
        </span>
      </span>
    </template>
  </RowLink>
</template>

<style scoped>
.lab-row :deep(.row-link__title) {
  text-transform: none;
  letter-spacing: -0.01em;
  font-size: 0.98rem;
}

.lab-row__name {
  display: inline-block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lab-row__subtitle {
  display: inline-flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 6px;
}

.lab-row__id {
  font-family: var(--font-mono);
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.lab-row__sep {
  opacity: 0.55;
}

.lab-row__meta {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.lab-row__metrics {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.lab-row__metric {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border: 1px solid var(--border-default);
  border-radius: 999px;
  background: color-mix(in srgb, var(--bg-surface) 80%, transparent);
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.7rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  white-space: nowrap;
}

.lab-row__metric--overflow {
  color: var(--text-tertiary);
}

.lab-row__metric-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.lab-row__metric-dot--amber {
  background: var(--tone-amber);
}

.lab-row__metric-dot--cyan {
  background: var(--tone-cyan);
}

.lab-row__metric-dot--purple {
  background: var(--tone-purple);
}
</style>
