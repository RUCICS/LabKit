<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { RouterLink } from 'vue-router';
import AdminShell from '../components/admin/AdminShell.vue';
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
  <AdminShell>
    <div class="admin-queue" data-testid="admin-queue">
      <nav class="admin-queue__breadcrumb" aria-label="breadcrumb">
        <RouterLink :to="{ name: 'admin-labs' }" class="admin-queue__breadcrumb-link">← Labs</RouterLink>
        <span class="admin-queue__breadcrumb-sep">/</span>
        <span class="admin-queue__breadcrumb-lab">{{ resolvedLabId }}</span>
        <span class="admin-queue__breadcrumb-sep">/</span>
        <span aria-current="page">Queue</span>
      </nav>

      <div class="admin-queue__panel">
        <div class="admin-queue__actions">
          <button type="button" class="button" :disabled="actionBusy !== ''" @click="triggerReevaluation">
            {{ actionBusy === 'reeval' ? 'Queueing…' : '↺ Reevaluate all' }}
          </button>
          <button type="button" class="button button--secondary" :disabled="actionBusy !== ''" @click="exportGrades">
            {{ actionBusy === 'export' ? 'Exporting…' : '↓ Export grades' }}
          </button>
          <button type="button" class="button button--secondary admin-queue__refresh" :disabled="loading" @click="loadQueue">↻ Refresh</button>
        </div>

        <p v-if="actionError" class="admin-queue__status admin-queue__status--error">
          {{ actionError }}
        </p>
        <p v-else-if="actionNotice" class="admin-queue__status admin-queue__status--success">
          {{ actionNotice }}
        </p>
        <p v-if="loading" class="admin-queue__status">Loading queue…</p>
        <p v-else-if="error" class="admin-queue__status">{{ error }}</p>
        <p v-else-if="!queue || queue.jobs.length === 0" class="admin-queue__status">
          No recent jobs.
        </p>
        <div v-else class="admin-queue__jobs">
          <div class="admin-queue__summary">
            <span>{{ queueStats.total }} jobs</span>
            <span>{{ queueStats.running }} running</span>
            <span>{{ queueStats.queued }} queued</span>
          </div>
          <div class="admin-queue__table" role="table" aria-label="Queue jobs">
            <div class="admin-queue__row admin-queue__row--head" role="row">
              <div class="admin-queue__cell admin-queue__cell--head" role="columnheader">
                Job
              </div>
              <div class="admin-queue__cell admin-queue__cell--head" role="columnheader">
                Status
              </div>
              <div class="admin-queue__cell admin-queue__cell--head" role="columnheader">
                Submission
              </div>
              <div class="admin-queue__cell admin-queue__cell--head" role="columnheader">
                User
              </div>
              <div class="admin-queue__cell admin-queue__cell--head" role="columnheader">
                Attempts
              </div>
              <div class="admin-queue__cell admin-queue__cell--head" role="columnheader">
                Worker
              </div>
              <div class="admin-queue__cell admin-queue__cell--head" role="columnheader">
                Available
              </div>
              <div class="admin-queue__cell admin-queue__cell--head" role="columnheader">
                Updated
              </div>
              <div class="admin-queue__cell admin-queue__cell--head" role="columnheader">
                Error
              </div>
            </div>

            <div
              v-for="job in queue.jobs"
              :key="job.id"
              class="admin-queue__row"
              role="row"
              :data-testid="`queue-row-${job.id}`"
            >
              <div class="admin-queue__cell admin-queue__job-id" role="cell">
                {{ job.id }}
              </div>
              <div class="admin-queue__cell" role="cell">
                <VerdictBadge :value="job.status" />
              </div>
              <div class="admin-queue__cell" role="cell">
                {{ job.submission_id }}
              </div>
              <div class="admin-queue__cell" role="cell">{{ job.user_id }}</div>
              <div class="admin-queue__cell admin-queue__number" role="cell">
                {{ job.attempts }}
              </div>
              <div class="admin-queue__cell admin-queue__mono" role="cell">
                {{ job.worker_id || '—' }}
              </div>
              <div class="admin-queue__cell admin-queue__mono" role="cell">
                {{ formatTime(job.available_at) }}
              </div>
              <div class="admin-queue__cell admin-queue__mono" role="cell">
                {{ formatTime(job.updated_at) }}
              </div>
              <div class="admin-queue__cell" role="cell">
                <details
                  v-if="job.last_error"
                  class="admin-queue__error-details"
                  :data-testid="`job-${job.id}-error`"
                >
                  <summary class="admin-queue__error-summary">Last error</summary>
                  <pre class="admin-queue__error">{{ job.last_error }}</pre>
                </details>
                <span v-else class="admin-queue__empty">—</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AdminShell>
</template>

<style scoped>
.admin-queue {
  display: grid;
  gap: 20px;
}

.admin-queue__breadcrumb {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.82rem;
  color: var(--text-tertiary);
  margin-bottom: 16px;
}

.admin-queue__breadcrumb-link {
  color: var(--accent-strong);
  text-decoration: none;
}

.admin-queue__breadcrumb-link:hover { text-decoration: underline; }

.admin-queue__breadcrumb-sep { color: var(--border-strong); }

.admin-queue__breadcrumb-lab {
  font-family: var(--font-mono);
  font-size: 0.78rem;
}

.admin-queue__panel {
  display: grid;
  gap: 14px;
  padding: 24px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
}

.admin-queue__status {
  margin: 0;
  color: var(--muted);
}

.admin-queue__status--error {
  color: var(--danger);
}

.admin-queue__status--success {
  color: var(--accent-strong);
}

.admin-queue__actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.admin-queue__refresh {
  margin-left: auto;
}

.admin-queue__summary {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.admin-queue__jobs {
  display: grid;
  gap: 14px;
}

.admin-queue__error {
  margin: 0;
}

.admin-queue__table {
  border: 1px solid var(--border-default);
  border-radius: 10px;
  overflow: hidden;
  background: var(--bg-elevated);
}

.admin-queue__row {
  display: grid;
  grid-template-columns: 1.2fr 0.8fr 1fr 0.6fr 0.6fr 1fr 1.1fr 1.1fr 1.2fr;
  align-items: start;
  gap: 12px;
  padding: 12px 14px;
  border-top: 1px solid var(--border-default);
}

.admin-queue__row:first-child {
  border-top: none;
}

.admin-queue__row--head {
  background: var(--bg-surface);
}

.admin-queue__row:not(.admin-queue__row--head):hover {
  background: color-mix(in srgb, var(--bg-elevated) 92%, var(--bg-root));
}

.admin-queue__cell {
  min-width: 0;
  font-family: var(--font-mono);
  font-size: 0.86rem;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}

.admin-queue__cell--head {
  color: var(--text-tertiary);
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.admin-queue__job-id {
  font-family: var(--font-mono);
  font-size: 0.9rem;
  font-weight: 600;
}

.admin-queue__mono {
  color: var(--text-secondary);
}

.admin-queue__number {
  text-align: right;
}

.admin-queue__empty {
  color: var(--text-tertiary);
}

.admin-queue__error-details {
  color: var(--danger);
}

.admin-queue__error-summary {
  cursor: pointer;
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  list-style: none;
}

.admin-queue__error-summary:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
  border-radius: 8px;
}

.admin-queue__error-summary::-webkit-details-marker {
  display: none;
}

.admin-queue__error-summary::before {
  content: '▸';
  display: inline-block;
  margin-right: 8px;
  transform: translateY(-0.5px);
}

.admin-queue__error-details[open] > .admin-queue__error-summary::before {
  content: '▾';
}

.admin-queue__error {
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
  .admin-queue__row {
    grid-template-columns: 1fr;
    gap: 10px;
  }

  .admin-queue__row--head {
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

  .admin-queue__number {
    text-align: left;
  }
}
</style>
