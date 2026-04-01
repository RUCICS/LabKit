<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { RouterLink } from 'vue-router';
import type { PublicLab } from '../components/board/types';

const labs = ref<PublicLab[]>([]);
const loading = ref(true);
const loadError = ref<string | null>(null);

async function loadLabs() {
  loading.value = true;
  loadError.value = null;
  try {
    const response = await fetch('/api/labs');
    if (!response.ok) {
      throw new Error(`Failed to load labs: ${response.status}`);
    }
    const payload = (await response.json()) as { labs: PublicLab[] };
    labs.value = payload.labs ?? [];
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : 'Failed to load labs';
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  void loadLabs();
});

function boardHref(labId: string) {
  return `/labs/${labId}/board`;
}

function labStatus(lab: PublicLab) {
  const now = Date.now();
  const open = Date.parse(lab.manifest?.schedule?.open ?? '');
  const close = Date.parse(lab.manifest?.schedule?.close ?? '');
  if (!Number.isNaN(open) && now < open) {
    return 'UPCOMING';
  }
  if (!Number.isNaN(close) && now > close) {
    return 'CLOSED';
  }
  return 'OPEN';
}

function metricDot(metricId: string) {
  const value = metricId.toLowerCase();
  if (value.includes('latency')) {
    return 'lab-card__metric-dot--latency';
  }
  if (value.includes('fair')) {
    return 'lab-card__metric-dot--fairness';
  }
  return 'lab-card__metric-dot--throughput';
}

function formatCloseDate(lab: PublicLab) {
  const close = Date.parse(lab.manifest?.schedule?.close ?? '');
  if (Number.isNaN(close)) {
    return 'TBD';
  }
  return new Intl.DateTimeFormat('en-US', {
    month: '2-digit',
    day: '2-digit'
  }).format(close);
}
</script>

<template>
  <main class="page-shell lab-list-view" data-testid="page-shell">
    <section class="lab-list">
      <div class="lab-list__header">
        <h1>Labs</h1>
        <p>{{ labs.length }} listed</p>
      </div>

      <p v-if="loading" class="lab-list__status">Loading public labs…</p>
      <p v-else-if="loadError" class="lab-list__status">{{ loadError }}</p>
      <p v-else-if="labs.length === 0" class="lab-list__status">No labs available.</p>

      <div v-else class="lab-grid">
        <RouterLink
          v-for="lab in labs"
          :key="lab.id"
          class="lab-card"
          :to="boardHref(lab.id)"
        >
          <div class="lab-card__top">
            <div class="lab-card__titles">
              <h3>{{ lab.name }}</h3>
              <p>{{ lab.id }}</p>
            </div>
            <span class="lab-card__status" :data-status="labStatus(lab)">
              <span class="lab-card__status-dot" />
              {{ labStatus(lab) }}
            </span>
          </div>
          <div class="lab-card__body">
            <p>
              {{ lab.manifest?.submit?.files?.[0] || 'submission' }} ·
              {{ lab.manifest?.metrics?.length ?? 0 }} metrics
            </p>
            <p>
              closes {{ formatCloseDate(lab) }}
            </p>
          </div>
          <div v-if="lab.manifest?.metrics?.length" class="lab-card__metrics">
            <span
              v-for="metric in lab.manifest.metrics"
              :key="metric.id"
              class="lab-card__metric"
            >
              <span class="lab-card__metric-dot" :class="metricDot(metric.id)" />
              {{ metric.name }}
            </span>
          </div>
        </RouterLink>
      </div>
    </section>
  </main>
</template>

<style scoped>
.lab-list-view {
  padding-top: 24px;
}

.lab-list {
  display: grid;
  gap: 16px;
}

.lab-list__header {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 16px;
}

.lab-list__header h1 {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 1.7rem;
  font-weight: 700;
  letter-spacing: -0.04em;
}

.lab-list__header p,
.lab-list__status {
  margin: 0;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.lab-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 16px;
}

.lab-card {
  display: grid;
  gap: 16px;
  padding: 20px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
  color: var(--text-primary);
  text-decoration: none;
  transition:
    transform 180ms ease,
    border-color 180ms ease;
}

.lab-card:hover {
  transform: translateY(-1px);
  border-color: var(--border-strong);
}

.lab-card__top {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: start;
}

.lab-card__titles {
  display: grid;
  gap: 4px;
}

.lab-card h3,
.lab-card p,
.lab-card__body p {
  margin: 0;
}

.lab-card h3 {
  font-size: 1rem;
  font-weight: 600;
}

.lab-card__titles p,
.lab-card__body p {
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.78rem;
}

.lab-card__status {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 5px 10px;
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.lab-card__status[data-status='OPEN'] {
  color: var(--color-open);
  background: rgba(52, 211, 153, 0.08);
}

.lab-card__status[data-status='UPCOMING'] {
  color: var(--text-secondary);
  background: var(--bg-elevated);
}

.lab-card__status[data-status='CLOSED'] {
  color: var(--text-tertiary);
  background: var(--bg-elevated);
}

.lab-card__status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.lab-card__body {
  display: grid;
  gap: 6px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-default);
}

.lab-card__metrics {
  display: flex;
  gap: 14px;
  flex-wrap: wrap;
}

.lab-card__metric {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
}

.lab-card__metric-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.lab-card__metric-dot--throughput {
  background: var(--track-throughput);
}

.lab-card__metric-dot--latency {
  background: var(--track-latency);
}

.lab-card__metric-dot--fairness {
  background: var(--track-fairness);
}

@media (max-width: 767px) {
  .lab-list-view {
    padding-top: 8px;
  }

  .lab-list__header {
    align-items: start;
    flex-direction: column;
  }

  .lab-grid {
    grid-template-columns: 1fr;
  }
}
</style>
