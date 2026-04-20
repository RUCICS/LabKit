<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { RouterLink, useRoute } from 'vue-router';
import type { PublicLab } from '../components/board/types';
import PageTitleBlock from '../components/chrome/PageTitleBlock.vue';
import SectionHeader from '../components/chrome/SectionHeader.vue';
import StatusBadge from '../components/chrome/StatusBadge.vue';
import {
  adminTokenStorageKey,
  authorizedAdminHeaders,
  readAPIError,
  writeAdminToken
} from '../lib/admin';
import { getLabPhase, labPhaseLabel } from '../lib/labs';

type AdminLab = PublicLab;

const labs = ref<AdminLab[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);
const formMode = ref<'register' | 'update'>('register');
const targetLabID = ref('');
const manifestBody = ref('');
const submitBusy = ref(false);
const submitError = ref<string | null>(null);
const submitNotice = ref<string | null>(null);
const route = useRoute();
const queueQuery = computed(() => {
  const { token, ...query } = route.query;
  return query;
});

const submitLabel = computed(() => (formMode.value === 'update' ? 'Update lab' : 'Register lab'));
const editorTitle = computed(() => (formMode.value === 'update' ? 'Update manifest' : 'Register manifest'));

function persistAdminToken() {
  const token = route.query.token;
  if (typeof token !== 'string' || token.trim() === '') {
    return;
  }
  writeAdminToken(token.trim());
  const url = new URL(window.location.href);
  url.searchParams.delete('token');
  window.history.replaceState({}, '', `${url.pathname}${url.search}${url.hash}`);
}

async function loadLabs() {
  loading.value = true;
  error.value = null;
  try {
    const response = await fetch('/api/labs');
    if (!response.ok) {
      throw new Error(`Failed to load labs: ${response.status}`);
    }
    const payload = (await response.json()) as { labs?: AdminLab[] };
    labs.value = payload.labs ?? [];
  } catch (requestError) {
    error.value = requestError instanceof Error ? requestError.message : 'Failed to load labs.';
  } finally {
    loading.value = false;
  }
}

function resetForm() {
  formMode.value = 'register';
  targetLabID.value = '';
  manifestBody.value = '';
  submitError.value = null;
  submitNotice.value = null;
}

function beginUpdate(labId: string) {
  formMode.value = 'update';
  targetLabID.value = labId;
  submitError.value = null;
  submitNotice.value = null;
}

function labPhase(lab: AdminLab) {
  return getLabPhase(lab.manifest?.schedule);
}

async function submitManifest() {
  submitBusy.value = true;
  submitError.value = null;
  submitNotice.value = null;

  const target = targetLabID.value.trim();
  const manifest = manifestBody.value;
  if (manifest.trim() === '') {
    submitBusy.value = false;
    submitError.value = 'Manifest is required.';
    return;
  }
  if (formMode.value === 'update' && target === '') {
    submitBusy.value = false;
    submitError.value = 'Lab ID is required for updates.';
    return;
  }

  const path = formMode.value === 'update' ? `/api/admin/labs/${encodeURIComponent(target)}` : '/api/admin/labs';
  const method = formMode.value === 'update' ? 'PUT' : 'POST';

  try {
    const response = await fetch(path, {
      method,
      headers: authorizedAdminHeaders({
        'Content-Type': 'text/plain; charset=utf-8'
      }),
      body: manifest
    });
    if (!response.ok) {
      throw new Error(await readAPIError(response, `Failed to ${formMode.value} lab.`));
    }
    const payload = (await response.json()) as { id?: string };
    submitNotice.value = formMode.value === 'update' ? 'Lab updated.' : 'Lab registered.';
    if (payload.id) {
      targetLabID.value = payload.id;
    }
    await loadLabs();
  } catch (requestError) {
    submitError.value =
      requestError instanceof Error ? requestError.message : `Failed to ${formMode.value} lab.`;
  } finally {
    submitBusy.value = false;
  }
}

onMounted(() => {
  persistAdminToken();
  void loadLabs();
});
</script>

<template>
  <main class="page-shell admin-labs-view" data-testid="page-shell">
    <PageTitleBlock title="Labs" eyebrow="Admin" lede="Register, update, and maintain labs.">
      <template #actions>
        <p class="admin-labs-view__token">
          Token key:
          <span>{{ adminTokenStorageKey }}</span>
        </p>
      </template>
    </PageTitleBlock>

    <section class="admin-labs-view__layout">
      <section class="admin-labs-view__panel">
        <SectionHeader
          title="Catalog"
          :subtitle="`${labs.length} lab${labs.length === 1 ? '' : 's'}`"
        >
          <template #actions>
            <button type="button" class="button button--secondary" @click="resetForm">New</button>
          </template>
        </SectionHeader>
        <p v-if="loading" class="admin-labs-view__status">Loading labs…</p>
        <p v-else-if="error" class="admin-labs-view__status">{{ error }}</p>
        <p v-else-if="labs.length === 0" class="admin-labs-view__status">No labs yet.</p>
        <div v-else class="admin-labs-view__rows" role="list">
          <article v-for="lab in labs" :key="lab.id" class="admin-labs-view__row" role="listitem">
            <div class="admin-labs-view__row-main">
              <div class="admin-labs-view__row-title">
                <h3 class="admin-labs-view__row-name">{{ lab.name }}</h3>
                <StatusBadge :label="labPhaseLabel(labPhase(lab))" :tone="labPhase(lab)" />
              </div>
              <p class="admin-labs-view__row-id">{{ lab.id }}</p>
            </div>

            <div class="admin-labs-view__row-meta">
              <p v-if="lab.manifest?.metrics?.length" class="admin-labs-view__row-metrics">
                {{ lab.manifest.metrics.length }} metrics
              </p>
              <p v-else class="admin-labs-view__row-metrics admin-labs-view__row-metrics--muted">
                No metrics
              </p>
            </div>

            <div class="admin-labs-view__row-actions" data-testid="lab-row-actions">
              <button type="button" class="button button--secondary" @click="beginUpdate(lab.id)">
                Edit
              </button>
              <RouterLink
                class="button button--secondary"
                :to="{ name: 'admin-queue', params: { labID: lab.id }, query: queueQuery }"
              >
                Queue
              </RouterLink>
            </div>
          </article>
        </div>
      </section>

      <section class="admin-labs-view__panel admin-labs-view__panel--editor">
        <SectionHeader
          :title="editorTitle"
          :subtitle="formMode === 'update' ? 'Update an existing lab.' : 'Register a new lab.'"
        />

        <div class="admin-labs-view__controls">
          <label class="field">
            <span>Action</span>
            <select v-model="formMode">
              <option value="register">Register</option>
              <option value="update">Update</option>
            </select>
          </label>

          <label class="field">
            <span>Lab ID</span>
            <input
              v-model="targetLabID"
              name="lab-id"
              type="text"
              placeholder="sorting"
              :disabled="formMode === 'register'"
            />
          </label>
        </div>

        <label class="field field--stacked">
          <span>Manifest</span>
          <textarea
            v-model="manifestBody"
            class="admin-labs-view__editor"
            spellcheck="false"
            placeholder="[lab]"
            rows="18"
          />
        </label>

        <p v-if="submitError" class="admin-labs-view__feedback admin-labs-view__feedback--error">
          {{ submitError }}
        </p>
        <p
          v-else-if="submitNotice"
          class="admin-labs-view__feedback admin-labs-view__feedback--success"
        >
          {{ submitNotice }}
        </p>

        <div class="admin-labs-view__submit">
          <button type="button" class="button" :disabled="submitBusy" @click="submitManifest">
            {{ submitBusy ? 'Saving…' : submitLabel }}
          </button>
        </div>
      </section>
    </section>
  </main>
</template>

<style scoped>
.admin-labs-view {
  display: grid;
  gap: 20px;
}

.admin-labs-view__token {
  margin: 0;
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.7rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  white-space: nowrap;
}

.admin-labs-view__token span {
  color: var(--text-secondary);
  text-transform: none;
  letter-spacing: 0.02em;
}

.admin-labs-view__layout {
  display: grid;
  grid-template-columns: minmax(280px, 1fr) minmax(380px, 1.2fr);
  gap: 20px;
}

.admin-labs-view__panel {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 24px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
}

.admin-labs-view__feedback {
  margin: 0;
}

.admin-labs-view__status {
  margin: 0;
  color: var(--muted);
}

.admin-labs-view__rows {
  display: grid;
  gap: 10px;
}

.admin-labs-view__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 14px 14px 14px 16px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-elevated);
  transition: border-color 150ms ease;
}

.admin-labs-view__row:hover {
  border-color: var(--border-strong);
}

.admin-labs-view__row-main {
  min-width: 0;
  display: grid;
  gap: 4px;
  flex: 1;
}

.admin-labs-view__row-title {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  min-width: 0;
}

.admin-labs-view__row-name {
  margin: 0;
  font-size: 0.98rem;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.admin-labs-view__row-id {
  margin: 0;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
}

.admin-labs-view__row-meta {
  flex: 0 0 auto;
  min-width: 120px;
  display: flex;
  justify-content: flex-end;
}

.admin-labs-view__row-metrics {
  margin: 0;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.admin-labs-view__row-metrics--muted {
  color: var(--text-tertiary);
}

.admin-labs-view__row-actions {
  display: flex;
  gap: 10px;
  align-items: center;
  flex: 0 0 auto;
}

.admin-labs-view__controls {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.admin-labs-view__editor {
  width: 100%;
  flex: 1;
  min-height: 360px;
  resize: vertical;
  font-family: var(--font-mono);
  font-size: 0.82rem;
  line-height: 1.5;
}

.admin-labs-view__feedback--error {
  color: var(--danger);
}

.admin-labs-view__feedback--success {
  color: var(--accent-strong);
}

.admin-labs-view__submit {
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 980px) {
  .admin-labs-view__layout {
    grid-template-columns: 1fr;
  }

  .admin-labs-view__controls {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 767px) {
  .admin-labs-view__row {
    align-items: flex-start;
    flex-direction: column;
  }

  .admin-labs-view__row-meta {
    justify-content: flex-start;
    min-width: 0;
  }
}
</style>
