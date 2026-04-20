<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import LeaderboardEmptyState from '../components/board/LeaderboardEmptyState.vue';
import LeaderboardHiddenState from '../components/board/LeaderboardHiddenState.vue';
import LeaderboardMetricTabs from '../components/board/LeaderboardMetricTabs.vue';
import LeaderboardTable from '../components/board/LeaderboardTable.vue';
import LabContextBar from '../components/chrome/LabContextBar.vue';
import QuotaSummaryBar from '../components/chrome/QuotaSummaryBar.vue';
import type { LeaderboardBoard, LeaderboardLabDetail } from '../components/board/types';
import { readAPIError } from '../lib/http';
import { metricAccentTokens } from '../lib/labs';

const props = defineProps<{
  labId: string;
}>();

const board = ref<LeaderboardBoard | null>(null);
const lab = ref<LeaderboardLabDetail | null>(null);
const loading = ref(true);
const hidden = ref(false);
const error = ref<string | null>(null);
const activeMetric = ref('');
let requestSeq = 0;
const tableSentinel = ref<HTMLElement | null>(null);
const contextSticky = ref(false);
let tableObserver: IntersectionObserver | null = null;

const showTabs = computed(() => (board.value?.metrics.length ?? 0) > 1);
const selectedMetric = computed(
  () => board.value?.metrics.find((metric) => metric.id === activeMetric.value) ?? null
);
const labTitle = computed(() => lab.value?.name?.trim() || formatLabTitle(props.labId));
const metricUnits = computed(() =>
  Object.fromEntries((lab.value?.manifest?.metrics ?? []).map((metric) => [metric.id, metric.unit ?? '']))
);

const closeAtMs = computed(() => Date.parse(lab.value?.manifest?.schedule?.close ?? ''));
const remainingValue = computed(() =>
  Number.isNaN(closeAtMs.value) ? '—' : formatRemaining(closeAtMs.value)
);
const closesValue = computed(() =>
  Number.isNaN(closeAtMs.value) ? '' : formatCloseDate(closeAtMs.value)
);
const participantsValue = computed(() => String(board.value?.rows.length ?? 0));
const lastUpdatedMs = computed(() => {
  const values = (board.value?.rows ?? [])
    .map((row) => Date.parse(row.updated_at))
    .filter((value) => !Number.isNaN(value))
    .sort((left, right) => right - left);
  return values[0] ?? null;
});
const lastUpdateValue = computed(() =>
  lastUpdatedMs.value === null ? '—' : formatContextTime(lastUpdatedMs.value)
);
const sortedByValue = computed(() => {
  if (!selectedMetric.value) {
    return '';
  }
  const arrow = selectedMetric.value.sort === 'asc' ? '↑' : '↓';
  return `${selectedMetric.value.name} ${arrow}`;
});

const accentStyle = computed(() => {
  const index = Math.max(board.value?.metrics.findIndex((m) => m.id === activeMetric.value) ?? 0, 0);
  const token = metricAccentTokens(index);
  return {
    '--accent': `var(${token.color})`,
    '--accent-dim': `var(${token.dim})`
  };
});

function metricIdFromLocation() {
  return new URLSearchParams(window.location.search).get('by')?.trim() ?? '';
}

function setMetricInLocation(metricId: string) {
  const url = new URL(window.location.href);
  if (metricId) {
    url.searchParams.set('by', metricId);
  } else {
    url.searchParams.delete('by');
  }
  window.history.replaceState({}, '', `${url.pathname}${url.search}${url.hash}`);
}

function formatLabTitle(labId: string) {
  const words = labId
    .replace(/[-_]+/g, ' ')
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1));
  return `${words.join(' ')} Lab`;
}

function formatRemaining(closeAt: number) {
  const remaining = Math.max(closeAt - Date.now(), 0);
  const days = Math.ceil(remaining / (1000 * 60 * 60 * 24));
  return `${days}d`;
}

function formatCloseDate(closeAt: number) {
  return new Intl.DateTimeFormat('en-US', {
    month: '2-digit',
    day: '2-digit'
  }).format(closeAt);
}

function formatContextTime(value: number) {
  return new Intl.DateTimeFormat('en-US', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(value);
}

async function loadBoard(metricId = metricIdFromLocation()) {
  const requestId = ++requestSeq;
  loading.value = true;
  hidden.value = false;
  error.value = null;
  try {
    const query = metricId ? `?by=${encodeURIComponent(metricId)}` : '';
    const [boardResponse, labResponse] = await Promise.all([
      fetch(`/api/labs/${encodeURIComponent(props.labId)}/board${query}`, { credentials: 'include' }),
      fetch(`/api/labs/${encodeURIComponent(props.labId)}`)
    ]);
    if (requestId !== requestSeq) {
      return;
    }
    if (boardResponse.status === 404) {
      const apiError = await readBoardError(boardResponse);
      if (requestId !== requestSeq) {
        return;
      }
      if (apiError?.code === 'lab_hidden') {
        board.value = null;
        hidden.value = true;
        return;
      }
      throw new Error(apiError?.message ?? 'Lab not found');
    }
    if (!boardResponse.ok) {
      throw new Error(await readAPIError(boardResponse, 'Failed to load leaderboard'));
    }
    if (!labResponse.ok && labResponse.status !== 404) {
      throw new Error(await readAPIError(labResponse, 'Failed to load lab details'));
    }
    board.value = (await boardResponse.json()) as LeaderboardBoard;
    lab.value = labResponse.ok ? ((await labResponse.json()) as LeaderboardLabDetail) : null;
    activeMetric.value = board.value.selected_metric;
    setMetricInLocation(board.value.selected_metric);
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

async function readBoardError(response: Response) {
  try {
    const payload = (await response.json()) as {
      error?: { code?: string; message?: string };
    };
    return payload.error ?? null;
  } catch {
    return null;
  }
}

function handleMetricSelect(metricId: string) {
  if (metricId === activeMetric.value) {
    return;
  }
  setMetricInLocation(metricId);
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

  if (typeof IntersectionObserver === 'undefined') {
    return;
  }

  tableObserver = new IntersectionObserver(
    (entries) => {
      const entry = entries[0];
      if (!entry) {
        return;
      }
      // We become "in table" once the sentinel has scrolled past the top edge.
      const scrolledPast = !entry.isIntersecting && entry.boundingClientRect.top < 0;
      contextSticky.value = scrolledPast;
    },
    {
      threshold: 0
    }
  );

  if (tableSentinel.value) {
    tableObserver.observe(tableSentinel.value);
  }
});

onBeforeUnmount(() => {
  tableObserver?.disconnect();
  tableObserver = null;
});
</script>

<template>
  <main class="page-shell leaderboard-view" :style="accentStyle">
    <LabContextBar
      v-if="board"
      :title="labTitle"
      :lab-id="board.lab_id"
      :remaining-value="remainingValue"
      :closes-at-value="closesValue"
      class="leaderboard-view__context"
      :class="{ 'leaderboard-view__context--sticky': contextSticky }"
    >
      <template #meta>
        <span>Participants {{ participantsValue }}</span>
        <span>Last update {{ lastUpdateValue }}</span>
        <span v-if="sortedByValue">Sorted by {{ sortedByValue }}</span>
      </template>
    </LabContextBar>

    <section class="leaderboard-view__content">
      <p v-if="loading" class="leaderboard-view__status">Loading leaderboard…</p>
      <p v-else-if="error" class="leaderboard-view__status">{{ error }}</p>
      <LeaderboardHiddenState v-else-if="hidden" />
      <template v-else-if="board">
        <div class="leaderboard-view__utility">
          <QuotaSummaryBar :quota="board.quota" />
          <a class="button button--secondary" :href="`/labs/${board.lab_id}/history`">My history</a>
        </div>

        <LeaderboardMetricTabs
          v-if="showTabs"
          :metrics="board.metrics"
          :selected-metric-id="activeMetric"
          @select="handleMetricSelect"
        />
        <div ref="tableSentinel" class="leaderboard-view__table-sentinel" aria-hidden="true" />
        <LeaderboardTable
          v-if="board.rows.length > 0 && selectedMetric"
          :rows="board.rows"
          :metrics="board.metrics"
          :selected-metric-id="activeMetric"
          :close-at="lab?.manifest?.schedule?.close"
          :api-hint="`GET /api/labs/${board.lab_id}/board`"
          :metric-units="metricUnits"
        />
        <LeaderboardEmptyState v-else />
      </template>
    </section>
  </main>
</template>

<style scoped>
.leaderboard-view {
  display: grid;
  gap: 16px;
}

.leaderboard-view__context {
  margin: 0;
}

.leaderboard-view__context--sticky {
  position: sticky;
  top: 0;
  z-index: 3;
  padding: 10px 12px;
  margin: 0 -12px;
  background: color-mix(in srgb, var(--bg-surface) 92%, transparent);
  border-bottom: 1px solid var(--border-default);
  backdrop-filter: blur(16px);
}

.leaderboard-view__table-sentinel {
  height: 1px;
}

.leaderboard-view__content {
  display: grid;
}

.leaderboard-view__content {
  gap: 12px;
}

.leaderboard-view__status {
  margin: 0;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.leaderboard-view__utility {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

@media (max-width: 767px) {
  .leaderboard-view__utility {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
