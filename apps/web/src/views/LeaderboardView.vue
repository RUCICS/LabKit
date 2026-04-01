<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import LeaderboardEmptyState from '../components/board/LeaderboardEmptyState.vue';
import LeaderboardHiddenState from '../components/board/LeaderboardHiddenState.vue';
import LeaderboardMetricTabs from '../components/board/LeaderboardMetricTabs.vue';
import LeaderboardTable from '../components/board/LeaderboardTable.vue';
import type { LeaderboardBoard, LeaderboardLabDetail } from '../components/board/types';

const props = defineProps<{
  labId: string;
}>();

const board = ref<LeaderboardBoard | null>(null);
const lab = ref<LeaderboardLabDetail | null>(null);
const loading = ref(true);
const hidden = ref(false);
const error = ref<string | null>(null);
const activeMetric = ref<string>('');
let requestSeq = 0;

const selectedMetric = computed(() => selectedMetricMetric());
const showTabs = computed(() => (board.value?.metrics.length ?? 0) > 1);
const labTitle = computed(() => lab.value?.name?.trim() || formatLabTitle(props.labId));
const statItems = computed(() => {
  const closeAt = Date.parse(lab.value?.manifest?.schedule?.close ?? '');
  return [
    {
      label: 'Participants',
      value: String(board.value?.rows.length ?? 0)
    },
    {
      label: 'Metrics',
      value: String(board.value?.metrics.length ?? 0)
    },
    {
      label: 'Remaining',
      value: Number.isNaN(closeAt) ? '—' : formatRemaining(closeAt)
    }
  ];
});
const accentStyle = computed(() => {
  const token = metricAccent(activeMetric.value || props.labId);
  return {
    '--accent': `var(${token.color})`,
    '--accent-dim': `var(${token.dim})`
  };
});

function formatLabTitle(labId: string) {
  const words = labId
    .replace(/[-_]+/g, ' ')
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1));
  return `${words.join(' ')} Lab`;
}

function metricAccent(metricId: string) {
  const value = metricId.toLowerCase();
  if (value.includes('latency')) {
    return { color: '--track-latency', dim: '--track-latency-dim' };
  }
  if (value.includes('fair')) {
    return { color: '--track-fairness', dim: '--track-fairness-dim' };
  }
  return { color: '--track-throughput', dim: '--track-throughput-dim' };
}

function selectedMetricMetric() {
  return board.value?.metrics.find((metric) => metric.id === activeMetric.value) ?? null;
}

async function loadBoard(metricId = '') {
  const requestId = ++requestSeq;
  loading.value = true;
  hidden.value = false;
  error.value = null;
  try {
    const query = metricId ? `?by=${encodeURIComponent(metricId)}` : '';
    const [boardResponse, labResponse] = await Promise.all([
      fetch(`/api/labs/${encodeURIComponent(props.labId)}/board${query}`),
      fetch(`/api/labs/${encodeURIComponent(props.labId)}`)
    ]);
    if (requestId !== requestSeq) {
      return;
    }
    if (boardResponse.status === 404) {
      const error = await readApiError(boardResponse);
      if (requestId !== requestSeq) {
        return;
      }
      if (error?.code === 'lab_hidden') {
        board.value = null;
        hidden.value = true;
        return;
      }
      throw new Error(error?.message ?? 'Lab not found');
    }
    if (!boardResponse.ok) {
      throw new Error(`Failed to load leaderboard: ${boardResponse.status}`);
    }
    if (!labResponse.ok && labResponse.status !== 404) {
      throw new Error(`Failed to load lab details: ${labResponse.status}`);
    }
    board.value = (await boardResponse.json()) as LeaderboardBoard;
    lab.value = labResponse.ok ? ((await labResponse.json()) as LeaderboardLabDetail) : null;
    activeMetric.value = board.value.selected_metric;
  } catch (requestError) {
    if (requestId !== requestSeq) {
      return;
    }
    error.value =
      requestError instanceof Error ? requestError.message : 'Failed to load leaderboard';
  } finally {
    if (requestId === requestSeq) {
      loading.value = false;
    }
  }
}

async function readApiError(response: Response) {
  try {
    const payload = (await response.json()) as {
      error?: { code?: string; message?: string };
    };
    return payload.error ?? null;
  } catch {
    return null;
  }
}

function formatRemaining(closeAt: number) {
  const remaining = Math.max(closeAt - Date.now(), 0);
  const days = Math.ceil(remaining / (1000 * 60 * 60 * 24));
  return `${days}d`;
}

function handleMetricSelect(metricId: string) {
  if (metricId === activeMetric.value) {
    return;
  }
  void loadBoard(metricId);
}

watch(
  () => props.labId,
  () => {
    void loadBoard();
  }
);

onMounted(() => {
  void loadBoard();
});
</script>

<template>
  <main class="page-shell leaderboard-view" :style="accentStyle">
    <section class="leaderboard-view__hero" v-if="board">
      <div class="leaderboard-view__title-block">
        <h1 class="leaderboard-view__title">{{ labTitle }}</h1>
        <p class="leaderboard-view__subtitle">{{ board.lab_id }}</p>
      </div>
      <div class="leaderboard-view__stats">
        <article
          v-for="item in statItems"
          :key="item.label"
          class="leaderboard-view__stat"
        >
          <strong>{{ item.value }}</strong>
          <span>{{ item.label }}</span>
        </article>
      </div>
    </section>

    <section class="leaderboard-view__content">
      <p v-if="loading" class="leaderboard-view__status">Loading leaderboard…</p>
      <p v-else-if="error" class="leaderboard-view__status">{{ error }}</p>
      <LeaderboardHiddenState v-else-if="hidden" />
      <template v-else-if="board">
        <LeaderboardMetricTabs
          v-if="showTabs"
          :metrics="board.metrics"
          :selected-metric-id="activeMetric"
          @select="handleMetricSelect"
        />
        <LeaderboardTable
          v-if="board.rows.length > 0 && selectedMetricMetric()"
          :rows="board.rows"
          :metrics="board.metrics"
          :selected-metric-id="activeMetric"
          :close-at="lab?.manifest?.schedule?.close"
          :api-hint="`GET /api/labs/${board.lab_id}/board`"
        />
        <LeaderboardEmptyState v-else />
      </template>
    </section>
  </main>
</template>

<style scoped>
.leaderboard-view {
  display: grid;
  gap: 24px;
}

.leaderboard-view__hero {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
}

.leaderboard-view__title-block {
  display: grid;
  gap: 10px;
}

.leaderboard-view__title {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 1.7rem;
  font-weight: 700;
  letter-spacing: -0.04em;
  line-height: 1;
}

.leaderboard-view__subtitle {
  margin: 0;
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.leaderboard-view__stats {
  display: flex;
  align-items: flex-end;
  gap: 24px;
  flex-wrap: wrap;
}

.leaderboard-view__stat {
  display: grid;
  gap: 6px;
}

.leaderboard-view__stat strong {
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 1.5rem;
  font-weight: 700;
  line-height: 1;
}

.leaderboard-view__stat span,
.leaderboard-view__status {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.leaderboard-view__content {
  display: grid;
  gap: 12px;
}

.leaderboard-view__status {
  color: var(--text-secondary);
}

@media (max-width: 767px) {
  .leaderboard-view__hero {
    flex-direction: column;
    align-items: flex-start;
  }

  .leaderboard-view__stats {
    width: 100%;
    justify-content: space-between;
    gap: 16px;
  }
}
</style>
