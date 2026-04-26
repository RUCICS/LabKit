<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import QuotaSummaryBar from '../components/chrome/QuotaSummaryBar.vue';
import StatusBadge from '../components/chrome/StatusBadge.vue';
import VerdictBadge from '../components/chrome/VerdictBadge.vue';
import type { LeaderboardLabDetail, QuotaSummary } from '../components/board/types';
import { readAPIError } from '../lib/http';
import { getLabPhase, getLabSchedule, labPhaseLabel } from '../lib/labs';

type HistoryItem = {
  id: string;
  status: string;
  verdict?: string;
  message?: string;
  created_at: string;
  finished_at?: string;
};

type SubmissionDetail = HistoryItem & {
  detail?: {
    format?: string;
    content?: string;
  };
  scores?: Array<{ metric_id: string; value: number }>;
  quota?: QuotaSummary;
};

const props = defineProps<{
  labId: string;
}>();

const history = ref<HistoryItem[]>([]);
const details = ref<Record<string, SubmissionDetail>>({});
const expandedId = ref('');
const loading = ref(true);
const error = ref<string | null>(null);
const detailError = ref<string | null>(null);
const detailLoadingId = ref('');
const quota = ref<QuotaSummary | null>(null);
const lab = ref<LeaderboardLabDetail | null>(null);

const labTitle = computed(() => lab.value?.name?.trim() || props.labId);
const phase = computed(() => (lab.value ? getLabPhase(getLabSchedule(lab.value.manifest)) : 'open'));

function formatTime(value?: string) {
  const date = new Date(value ?? '');
  if (Number.isNaN(date.getTime())) {
    return '—';
  }
  return new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit'
  }).format(date);
}

function formatScore(value: number) {
  return new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(value);
}

function detailToggleLabel(item: HistoryItem) {
  if (expandedId.value === item.id) {
    return 'Hide detail';
  }
  return 'Expand detail';
}

function escapeHTML(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function renderDetail(detail?: SubmissionDetail['detail']) {
  if (!detail?.content) {
    return '';
  }
  const safe = escapeHTML(detail.content);
  if (detail.format !== 'markdown') {
    return `<pre>${safe}</pre>`;
  }
  return safe
    .split(/\n{2,}/)
    .map((block) => {
      const trimmed = block.trim();
      if (trimmed.startsWith('### ')) {
        return `<h3>${trimmed.slice(4)}</h3>`;
      }
      if (trimmed.startsWith('## ')) {
        return `<h2>${trimmed.slice(3)}</h2>`;
      }
      if (trimmed.startsWith('# ')) {
        return `<h1>${trimmed.slice(2)}</h1>`;
      }
      if (trimmed.includes('|')) {
        return `<pre>${trimmed}</pre>`;
      }
      return `<p>${trimmed.replaceAll('\n', '<br>')}</p>`;
    })
    .join('');
}

async function loadHistory() {
  loading.value = true;
  error.value = null;
  try {
    const [historyResponse, labResponse] = await Promise.all([
      fetch(`/api/labs/${encodeURIComponent(props.labId)}/history`, { credentials: 'include' }),
      fetch(`/api/labs/${encodeURIComponent(props.labId)}`)
    ]);
    if (!historyResponse.ok) {
      throw new Error(await readAPIError(historyResponse, 'Failed to load history'));
    }
    if (!labResponse.ok) {
      throw new Error(await readAPIError(labResponse, 'Failed to load lab'));
    }
    const historyPayload = (await historyResponse.json()) as {
      submissions?: HistoryItem[];
      quota?: QuotaSummary;
    };
    history.value = historyPayload.submissions ?? [];
    quota.value = historyPayload.quota ?? null;
    lab.value = (await labResponse.json()) as LeaderboardLabDetail;
  } catch (requestError) {
    error.value = requestError instanceof Error ? requestError.message : 'Failed to load history';
  } finally {
    loading.value = false;
  }
}

async function toggleDetail(item: HistoryItem) {
  if (expandedId.value === item.id) {
    expandedId.value = '';
    return;
  }
  expandedId.value = item.id;
  detailError.value = null;
  if (details.value[item.id]) {
    return;
  }
  detailLoadingId.value = item.id;
  try {
    const response = await fetch(`/api/labs/${encodeURIComponent(props.labId)}/submissions/${item.id}`, {
      credentials: 'include'
    });
    if (!response.ok) {
      throw new Error(await readAPIError(response, 'Failed to load submission detail'));
    }
    details.value = {
      ...details.value,
      [item.id]: (await response.json()) as SubmissionDetail
    };
  } catch (requestError) {
    detailError.value =
      requestError instanceof Error ? requestError.message : 'Failed to load submission detail';
  } finally {
    detailLoadingId.value = '';
  }
}

onMounted(() => {
  void loadHistory();
});
</script>

<template>
  <main class="page-shell history-view" data-testid="page-shell">
    <section class="history-view__hero" v-if="lab">
      <div class="history-view__title-block">
        <div class="history-view__eyebrow">
          <span>My submissions</span>
          <StatusBadge :label="labPhaseLabel(phase)" :tone="phase" />
        </div>
        <h1 class="history-view__title">{{ labTitle }}</h1>
        <p class="history-view__subtitle">{{ props.labId }}</p>
      </div>
      <div class="history-view__meta">
        <QuotaSummaryBar :quota="quota" />
        <a class="button button--secondary" :href="`/labs/${props.labId}/board`">View board</a>
      </div>
    </section>

    <section class="history-view__content">
      <p v-if="loading" class="history-view__status">Loading submission history…</p>
      <p v-else-if="error" class="history-view__status">{{ error }}</p>
      <p v-else-if="history.length === 0" class="history-view__status">No submissions yet.</p>
      <div v-else class="history-view__timeline">
        <article v-for="item in history" :key="item.id" class="history-view__item">
          <div class="history-view__item-head">
            <div class="history-view__item-copy">
              <h2>Submission {{ item.id.slice(0, 8) }}</h2>
              <p>{{ formatTime(item.finished_at || item.created_at) }}</p>
            </div>
            <VerdictBadge :value="item.verdict || item.status" />
          </div>

          <p v-if="item.message" class="history-view__message">{{ item.message }}</p>

          <div class="history-view__actions">
            <button
              type="button"
              class="button button--secondary"
              :disabled="detailLoadingId === item.id"
              @click="toggleDetail(item)"
            >
              {{ detailLoadingId === item.id ? 'Loading…' : detailToggleLabel(item) }}
            </button>
          </div>

          <div v-if="expandedId === item.id" class="history-view__detail">
            <p v-if="detailError" class="history-view__status">{{ detailError }}</p>
            <template v-else-if="details[item.id]">
              <dl v-if="details[item.id].scores?.length" class="history-view__scores">
                <div v-for="score in details[item.id].scores" :key="score.metric_id">
                  <dt>{{ score.metric_id }}</dt>
                  <dd>{{ formatScore(score.value) }}</dd>
                </div>
              </dl>
              <div
                v-if="details[item.id].detail?.content"
                class="history-view__detail-rendered"
                v-html="renderDetail(details[item.id].detail)"
              />
            </template>
          </div>
        </article>
      </div>
    </section>
  </main>
</template>

<style scoped>
.history-view {
  display: grid;
  gap: 24px;
}

.history-view__hero {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 24px;
}

.history-view__title-block,
.history-view__meta,
.history-view__timeline,
.history-view__item,
.history-view__detail,
.history-view__scores {
  display: grid;
}

.history-view__title-block {
  gap: 10px;
}

.history-view__eyebrow {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.history-view__title {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 1.7rem;
  font-weight: 700;
  letter-spacing: -0.04em;
}

.history-view__status,
.history-view__item-copy p,
.history-view__message {
  margin: 0;
  color: var(--text-secondary);
}

.history-view__subtitle {
  margin: 0;
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.history-view__meta {
  gap: 12px;
  justify-items: end;
}

.history-view__timeline {
  gap: 16px;
}

.history-view__item {
  gap: 14px;
  padding: 18px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
  transition: border-color 150ms ease;
}

.history-view__item:hover {
  border-color: var(--border-strong);
}

.history-view__item-head {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 16px;
}

.history-view__item-copy {
  display: grid;
  gap: 4px;
}

.history-view__item-copy h2 {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 0.95rem;
}

.history-view__actions {
  display: flex;
  justify-content: flex-start;
}

.history-view__detail {
  gap: 16px;
  padding-top: 4px;
}

.history-view__scores {
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 12px;
}

.history-view__scores div {
  display: grid;
  gap: 4px;
  padding: 12px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-elevated);
}

.history-view__scores dt,
.history-view__scores dd {
  margin: 0;
}

.history-view__scores dt {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.history-view__scores dd {
  font-family: var(--font-mono);
  font-size: 0.95rem;
  font-variant-numeric: tabular-nums;
}

.history-view__detail-rendered :deep(h1),
.history-view__detail-rendered :deep(h2),
.history-view__detail-rendered :deep(h3),
.history-view__detail-rendered :deep(p),
.history-view__detail-rendered :deep(pre) {
  margin: 0 0 12px;
}

.history-view__detail-rendered :deep(h1),
.history-view__detail-rendered :deep(h2),
.history-view__detail-rendered :deep(h3) {
  font-family: var(--font-mono);
}

.history-view__detail-rendered :deep(pre) {
  overflow: auto;
  padding: 12px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-elevated);
  font-family: var(--font-mono);
  white-space: pre-wrap;
}

@media (max-width: 767px) {
  .history-view__hero,
  .history-view__item-head {
    flex-direction: column;
    align-items: start;
  }

  .history-view__meta {
    justify-items: start;
  }
}
</style>
