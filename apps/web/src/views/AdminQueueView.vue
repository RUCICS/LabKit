<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import VerdictBadge from '../components/chrome/VerdictBadge.vue';
import {
  authorizedAdminHeaders,
  fileNameFromDisposition,
  readAPIError
} from '../lib/admin';

type QueueJob = {
  id: string;
  submission_id: string;
  user_id: number;
  status: string;
  attempts: number;
  available_at: string;
  worker_id?: string;
  last_error?: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
  updated_at: string;
};

type QueueStatus = {
  lab_id: string;
  jobs: QueueJob[];
};

const props = defineProps<{
  labId?: string;
}>();

const queue = ref<QueueStatus | null>(null);
const loading = ref(true);
const error = ref<string | null>(null);
const actionBusy = ref<'reeval' | 'export' | ''>('');
const actionError = ref<string | null>(null);
const actionNotice = ref<string | null>(null);
let requestSeq = 0;

const resolvedLabId = computed(() => props.labId ?? labIdFromPath(window.location.pathname));
const queueStats = computed(() => {
  const jobs = queue.value?.jobs ?? [];
  return {
    total: jobs.length,
    running: jobs.filter((job) => job.status === 'running').length,
    queued: jobs.filter((job) => job.status === 'queued').length
  };
});

function labIdFromPath(pathname: string) {
  const match = pathname.match(/\/admin\/labs\/([^/]+)\/queue/);
  return match?.[1] ?? '';
}

function formatTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value || '—';
  }
  return new Intl.DateTimeFormat('en-US', {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(date);
}

async function loadQueue() {
  const requestId = ++requestSeq;
  const labId = resolvedLabId.value;
  loading.value = true;
  error.value = null;
  queue.value = null;

  if (!labId) {
    error.value = 'Missing lab ID.';
    loading.value = false;
    return;
  }

  try {
    const response = await fetch(`/api/admin/labs/${encodeURIComponent(labId)}/queue`, {
      headers: authorizedAdminHeaders()
    });
    if (requestId !== requestSeq) {
      return;
    }
    if (!response.ok) {
      throw new Error(`Failed to load queue: ${response.status}`);
    }
    queue.value = (await response.json()) as QueueStatus;
  } catch (requestError) {
    if (requestId !== requestSeq) {
      return;
    }
    error.value = requestError instanceof Error ? requestError.message : 'Failed to load queue.';
  } finally {
    if (requestId === requestSeq) {
      loading.value = false;
    }
  }
}

async function triggerReevaluation() {
  if (!resolvedLabId.value) {
    return;
  }
  actionBusy.value = 'reeval';
  actionError.value = null;
  actionNotice.value = null;
  try {
    const response = await fetch(`/api/admin/labs/${encodeURIComponent(resolvedLabId.value)}/reeval`, {
      method: 'POST',
      headers: authorizedAdminHeaders()
    });
    if (!response.ok) {
      throw new Error(await readAPIError(response, 'Failed to queue re-evaluation.'));
    }
    const payload = (await response.json()) as { jobs_created?: number };
    const count = payload.jobs_created ?? 0;
    actionNotice.value = `Queued ${count} re-evaluations.`;
    await loadQueue();
  } catch (requestError) {
    actionError.value =
      requestError instanceof Error ? requestError.message : 'Failed to queue re-evaluation.';
  } finally {
    actionBusy.value = '';
  }
}

async function exportGrades() {
  if (!resolvedLabId.value) {
    return;
  }
  actionBusy.value = 'export';
  actionError.value = null;
  actionNotice.value = null;
  try {
    const response = await fetch(`/api/admin/labs/${encodeURIComponent(resolvedLabId.value)}/grades`, {
      headers: authorizedAdminHeaders()
    });
    if (!response.ok) {
      throw new Error(await readAPIError(response, 'Failed to export grades.'));
    }
    const blob = await response.blob();
    const objectURL = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = objectURL;
    link.download = fileNameFromDisposition(
      response.headers.get('Content-Disposition'),
      `${resolvedLabId.value}-grades.csv`
    );
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(objectURL);
    actionNotice.value = 'Grades exported.';
  } catch (requestError) {
    actionError.value =
      requestError instanceof Error ? requestError.message : 'Failed to export grades.';
  } finally {
    actionBusy.value = '';
  }
}

watch(
  () => resolvedLabId.value,
  () => {
    void loadQueue();
  }
);

onMounted(() => {
  void loadQueue();
});
</script>

<template>
  <main class="page-shell admin-queue-view" data-testid="page-shell">
    <section class="admin-queue-view__header">
      <div class="admin-queue-view__header-copy">
        <h1>Queue status</h1>
        <p v-if="resolvedLabId" class="admin-queue-view__lab">{{ resolvedLabId }}</p>
      </div>
      <span class="admin-queue-view__meta">Admin controls</span>
    </section>

    <section class="admin-queue-view__panel">
      <div class="admin-queue-view__actions">
        <button
          type="button"
          class="button"
          :disabled="actionBusy !== ''"
          @click="triggerReevaluation"
        >
          {{ actionBusy === 'reeval' ? 'Queueing…' : 'Reevaluate' }}
        </button>
        <button
          type="button"
          class="button button--secondary"
          :disabled="actionBusy !== ''"
          @click="exportGrades"
        >
          {{ actionBusy === 'export' ? 'Exporting…' : 'Export grades' }}
        </button>
      </div>

      <p v-if="actionError" class="admin-queue-view__status admin-queue-view__status--error">
        {{ actionError }}
      </p>
      <p v-else-if="actionNotice" class="admin-queue-view__status admin-queue-view__status--success">
        {{ actionNotice }}
      </p>
      <p v-if="loading" class="admin-queue-view__status">Loading queue…</p>
      <p v-else-if="error" class="admin-queue-view__status">{{ error }}</p>
      <p v-else-if="!queue || queue.jobs.length === 0" class="admin-queue-view__status">
        No recent jobs.
      </p>
      <div v-else class="admin-queue-view__jobs">
        <div class="admin-queue-view__summary">
          <span>{{ queueStats.total }} jobs</span>
          <span>{{ queueStats.running }} running</span>
          <span>{{ queueStats.queued }} queued</span>
        </div>
        <div class="admin-queue-view__table" role="table" aria-label="Queue jobs">
          <div class="admin-queue-view__row admin-queue-view__row--head" role="row">
            <div class="admin-queue-view__cell admin-queue-view__cell--head" role="columnheader">
              Job
            </div>
            <div class="admin-queue-view__cell admin-queue-view__cell--head" role="columnheader">
              Status
            </div>
            <div class="admin-queue-view__cell admin-queue-view__cell--head" role="columnheader">
              Submission
            </div>
            <div class="admin-queue-view__cell admin-queue-view__cell--head" role="columnheader">
              User
            </div>
            <div class="admin-queue-view__cell admin-queue-view__cell--head" role="columnheader">
              Attempts
            </div>
            <div class="admin-queue-view__cell admin-queue-view__cell--head" role="columnheader">
              Worker
            </div>
            <div class="admin-queue-view__cell admin-queue-view__cell--head" role="columnheader">
              Available
            </div>
            <div class="admin-queue-view__cell admin-queue-view__cell--head" role="columnheader">
              Updated
            </div>
            <div class="admin-queue-view__cell admin-queue-view__cell--head" role="columnheader">
              Error
            </div>
          </div>

          <div
            v-for="job in queue.jobs"
            :key="job.id"
            class="admin-queue-view__row"
            role="row"
            :data-testid="`queue-row-${job.id}`"
          >
            <div class="admin-queue-view__cell admin-queue-view__job-id" role="cell">
              {{ job.id }}
            </div>
            <div class="admin-queue-view__cell" role="cell">
              <VerdictBadge :value="job.status" />
            </div>
            <div class="admin-queue-view__cell" role="cell">
              {{ job.submission_id }}
            </div>
            <div class="admin-queue-view__cell" role="cell">{{ job.user_id }}</div>
            <div class="admin-queue-view__cell admin-queue-view__number" role="cell">
              {{ job.attempts }}
            </div>
            <div class="admin-queue-view__cell admin-queue-view__mono" role="cell">
              {{ job.worker_id || '—' }}
            </div>
            <div class="admin-queue-view__cell admin-queue-view__mono" role="cell">
              {{ formatTime(job.available_at) }}
            </div>
            <div class="admin-queue-view__cell admin-queue-view__mono" role="cell">
              {{ formatTime(job.updated_at) }}
            </div>
            <div class="admin-queue-view__cell" role="cell">
              <details
                v-if="job.last_error"
                class="admin-queue-view__error-details"
                :data-testid="`job-${job.id}-error`"
              >
                <summary class="admin-queue-view__error-summary">Last error</summary>
                <pre class="admin-queue-view__error">{{ job.last_error }}</pre>
              </details>
              <span v-else class="admin-queue-view__empty">—</span>
            </div>
          </div>
        </div>
      </div>
    </section>
  </main>
</template>

<style scoped>
.admin-queue-view {
  display: grid;
  gap: 20px;
}

.admin-queue-view__header {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 16px;
}

.admin-queue-view__header-copy {
  display: grid;
  gap: 8px;
}

.admin-queue-view__header h1,
.admin-queue-view__meta,
.admin-queue-view__lab {
  margin: 0;
}

.admin-queue-view__header h1 {
  font-family: var(--font-mono);
  font-size: 1.7rem;
  font-weight: 700;
  letter-spacing: -0.04em;
}

.admin-queue-view__meta,
.admin-queue-view__lab {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.admin-queue-view__panel {
  display: grid;
  gap: 14px;
  padding: 24px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
}

.admin-queue-view__lab {
  margin: 0;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-weight: 600;
  font-size: 0.78rem;
  text-transform: uppercase;
}

.admin-queue-view__status {
  margin: 0;
  color: var(--muted);
}

.admin-queue-view__status--error {
  color: var(--danger);
}

.admin-queue-view__status--success {
  color: var(--accent-strong);
}

.admin-queue-view__actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.admin-queue-view__summary {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.admin-queue-view__jobs {
  display: grid;
  gap: 14px;
}

.admin-queue-view__error {
  margin: 0;
}

.admin-queue-view__table {
  border: 1px solid var(--border-default);
  border-radius: 10px;
  overflow: hidden;
  background: var(--bg-elevated);
}

.admin-queue-view__row {
  display: grid;
  grid-template-columns: 1.2fr 0.8fr 1fr 0.6fr 0.6fr 1fr 1.1fr 1.1fr 1.2fr;
  align-items: start;
  gap: 12px;
  padding: 12px 14px;
  border-top: 1px solid var(--border-default);
}

.admin-queue-view__row:first-child {
  border-top: none;
}

.admin-queue-view__row--head {
  background: var(--bg-surface);
}

.admin-queue-view__row:not(.admin-queue-view__row--head):hover {
  background: color-mix(in srgb, var(--bg-elevated) 92%, var(--bg-root));
}

.admin-queue-view__cell {
  min-width: 0;
  font-family: var(--font-mono);
  font-size: 0.86rem;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}

.admin-queue-view__cell--head {
  color: var(--text-tertiary);
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.admin-queue-view__job-id {
  font-family: var(--font-mono);
  font-size: 0.9rem;
  font-weight: 600;
}

.admin-queue-view__mono {
  color: var(--text-secondary);
}

.admin-queue-view__number {
  text-align: right;
}

.admin-queue-view__empty {
  color: var(--text-tertiary);
}

.admin-queue-view__error-details {
  color: var(--danger);
}

.admin-queue-view__error-summary {
  cursor: pointer;
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  list-style: none;
}

.admin-queue-view__error-summary:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
  border-radius: 8px;
}

.admin-queue-view__error-summary::-webkit-details-marker {
  display: none;
}

.admin-queue-view__error-summary::before {
  content: '▸';
  display: inline-block;
  margin-right: 8px;
  transform: translateY(-0.5px);
}

.admin-queue-view__error-details[open] > .admin-queue-view__error-summary::before {
  content: '▾';
}

.admin-queue-view__error {
  color: var(--danger);
  font-family: var(--font-mono);
  font-size: 0.8rem;
  line-height: 1.5;
  margin-top: 8px;
  padding: 10px;
  border: 1px solid color-mix(in srgb, var(--border-default) 70%, var(--danger));
  border-radius: 8px;
  background: color-mix(in srgb, var(--bg-root) 92%, var(--danger));
  white-space: pre-wrap;
  overflow: auto;
}

@media (max-width: 767px) {
  .admin-queue-view__header {
    flex-direction: column;
    align-items: start;
  }

  .admin-queue-view__row {
    grid-template-columns: 1fr;
    gap: 10px;
  }

  .admin-queue-view__row--head {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  .admin-queue-view__number {
    text-align: left;
  }
}
</style>
