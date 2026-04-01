<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { RouterLink, useRoute } from 'vue-router';
import {
  adminTokenStorageKey,
  authorizedAdminHeaders,
  readAPIError,
  writeAdminToken
} from '../lib/admin';

type AdminLab = {
  id: string;
  name: string;
};

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
    <section class="admin-labs-view__header">
      <h1>Labs</h1>
      <p>Admin catalog</p>
    </section>

    <section class="admin-labs-view__layout">
      <section class="admin-labs-view__panel">
        <div class="admin-labs-view__section-head">
          <div>
            <h2>Catalog</h2>
            <p>{{ labs.length }} lab<span v-if="labs.length !== 1">s</span></p>
          </div>
          <button type="button" class="button button--secondary" @click="resetForm">New</button>
        </div>
        <p v-if="loading" class="admin-labs-view__status">Loading labs…</p>
        <p v-else-if="error" class="admin-labs-view__status">{{ error }}</p>
        <p v-else-if="labs.length === 0" class="admin-labs-view__status">No labs yet.</p>
        <div v-else class="admin-labs-view__grid">
          <article v-for="lab in labs" :key="lab.id" class="admin-labs-view__card">
            <div class="admin-labs-view__card-copy">
              <h3>{{ lab.name }}</h3>
              <p>{{ lab.id }}</p>
            </div>
            <div class="admin-labs-view__card-actions">
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
        <div class="admin-labs-view__section-head">
          <div>
            <h2>{{ editorTitle }}</h2>
            <p>{{ formMode === 'update' ? 'Update an existing lab.' : 'Register a new lab.' }}</p>
          </div>
        </div>

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

.admin-labs-view__header {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 16px;
}

.admin-labs-view__header h1,
.admin-labs-view__header p {
  margin: 0;
}

.admin-labs-view__header h1 {
  font-family: var(--font-mono);
  font-size: 1.7rem;
  font-weight: 700;
  letter-spacing: -0.04em;
}

.admin-labs-view__header p {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.admin-labs-view__layout {
  display: grid;
  grid-template-columns: minmax(280px, 1fr) minmax(380px, 1.2fr);
  gap: 20px;
}

.admin-labs-view__panel {
  display: grid;
  gap: 18px;
  padding: 24px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
}

.admin-labs-view__section-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: start;
}

.admin-labs-view__section-head h2,
.admin-labs-view__section-head p,
.admin-labs-view__feedback {
  margin: 0;
}

.admin-labs-view__section-head h2 {
  font-size: 1.1rem;
}

.admin-labs-view__section-head p {
  margin-top: 6px;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.06em;
}

.admin-labs-view__status {
  margin: 0;
  color: var(--muted);
}

.admin-labs-view__grid {
  display: grid;
  gap: 14px;
}

.admin-labs-view__card {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 18px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-elevated);
}

.admin-labs-view__card-copy h3,
.admin-labs-view__card-copy p {
  margin: 0;
}

.admin-labs-view__card-copy h3 {
  font-size: 1rem;
}

.admin-labs-view__card-copy p {
  margin-top: 4px;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
}

.admin-labs-view__card-actions {
  display: flex;
  gap: 10px;
  align-items: start;
}

.admin-labs-view__controls {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.field {
  display: grid;
  gap: 8px;
}

.field span {
  color: var(--text-tertiary);
  font-size: 0.68rem;
  font-family: var(--font-mono);
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.field input,
.field select,
.field textarea {
  width: 100%;
  border: 1px solid var(--border-default);
  border-radius: 6px;
  background: var(--bg-elevated);
  color: var(--text-primary);
  padding: 13px 14px;
}

.field textarea {
  min-height: 320px;
  resize: vertical;
  font-family: var(--font-mono);
  line-height: 1.55;
}

.field--stacked {
  gap: 10px;
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
  .admin-labs-view__header {
    flex-direction: column;
    align-items: start;
  }
}
</style>
