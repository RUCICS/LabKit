<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
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
        <article v-for="job in queue.jobs" :key="job.id" class="admin-queue-view__job">
          <div class="admin-queue-view__job-head">
            <h2>{{ job.id }}</h2>
            <span>{{ job.status }}</span>
          </div>
          <dl class="admin-queue-view__grid">
            <div>
              <dt>Submission</dt>
              <dd>{{ job.submission_id }}</dd>
            </div>
            <div>
              <dt>User</dt>
              <dd>{{ job.user_id }}</dd>
            </div>
            <div>
              <dt>Attempts</dt>
              <dd>{{ job.attempts }}</dd>
            </div>
            <div>
              <dt>Worker</dt>
              <dd>{{ job.worker_id || '—' }}</dd>
            </div>
            <div>
              <dt>Available</dt>
              <dd>{{ formatTime(job.available_at) }}</dd>
            </div>
            <div>
              <dt>Updated</dt>
              <dd>{{ formatTime(job.updated_at) }}</dd>
            </div>
          </dl>
          <p v-if="job.last_error" class="admin-queue-view__error">{{ job.last_error }}</p>
        </article>
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

.admin-queue-view__jobs {
  display: grid;
  gap: 14px;
}

.admin-queue-view__job {
  display: grid;
  gap: 14px;
  padding: 16px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-elevated);
}

.admin-queue-view__job-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: baseline;
}

.admin-queue-view__job-head h2,
.admin-queue-view__job-head span,
.admin-queue-view__grid dt,
.admin-queue-view__grid dd,
.admin-queue-view__error {
  margin: 0;
}

.admin-queue-view__job-head span {
  color: var(--accent);
  text-transform: uppercase;
  letter-spacing: 0.12em;
  font-size: 0.75rem;
  font-weight: 700;
}

.admin-queue-view__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
}

.admin-queue-view__grid dt {
  color: var(--muted);
  font-size: 0.85rem;
}

.admin-queue-view__grid dd {
  margin-top: 4px;
  overflow-wrap: anywhere;
}

.admin-queue-view__error {
  color: var(--danger);
}

@media (max-width: 767px) {
  .admin-queue-view__header {
    flex-direction: column;
    align-items: start;
  }
}
</style>
