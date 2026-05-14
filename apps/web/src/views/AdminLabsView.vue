<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { RouterLink } from 'vue-router';
import AdminShell from '../components/admin/AdminShell.vue';
import LabEditDrawer from '../components/admin/LabEditDrawer.vue';
import StatusBadge from '../components/chrome/StatusBadge.vue';
import { authorizedAdminHeaders, readAPIError } from '../lib/admin';
import { getLabPhase, getLabSchedule, labPhaseLabel } from '../lib/labs';
import type { PublicLab } from '../components/board/types';

const labs = ref<PublicLab[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);
const drawerLabId = ref<string | null>(null);
const drawerOpen = ref(false);
const resetting = ref<string | null>(null);
const resetError = ref<string | null>(null);

async function loadLabs() {
  loading.value = true;
  error.value = null;
  try {
    const res = await fetch('/api/labs', { headers: authorizedAdminHeaders() });
    if (!res.ok) throw new Error(await readAPIError(res, 'Failed to load labs.'));
    const payload = (await res.json()) as { labs?: PublicLab[] };
    labs.value = payload.labs ?? [];
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load labs.';
  } finally {
    loading.value = false;
  }
}

function openEdit(labId: string | null) {
  drawerLabId.value = labId;
  drawerOpen.value = true;
}

function onDrawerClose() {
  drawerOpen.value = false;
}

async function onDrawerSaved() {
  drawerOpen.value = false;
  await loadLabs();
}

function labPhase(lab: PublicLab) {
  return getLabPhase(getLabSchedule(lab.manifest));
}

onMounted(() => void loadLabs());

async function resetQuota(labId: string, labName: string) {
  if (!confirm(`Reset daily quota for all users in "${labName}"?\n\nThis frees today's quota usage for everyone in this lab. Submissions remain in the audit log.`)) {
    return;
  }
  resetting.value = labId;
  resetError.value = null;
  try {
    const res = await fetch(`/api/admin/labs/${labId}/quota/reset`, {
      method: 'POST',
      headers: authorizedAdminHeaders(),
    });
    if (!res.ok) {
      resetError.value = await readAPIError(res, 'Failed to reset quota');
      return;
    }
    const data = (await res.json()) as { rows_affected: number };
    alert(`Quota reset. ${data.rows_affected} submission(s) cleared.`);
  } catch {
    resetError.value = 'Network error — quota reset failed';
  } finally {
    resetting.value = null;
  }
}
</script>

<template>
  <AdminShell>
    <div class="admin-labs" data-testid="admin-labs">
      <div class="admin-labs__header">
        <h1 class="admin-labs__title">Labs</h1>
        <button type="button" class="button" @click="openEdit(null)">+ New lab</button>
      </div>

      <p v-if="loading" class="admin-labs__status">Loading…</p>
      <p v-else-if="error" class="admin-labs__status admin-labs__status--error">{{ error }}</p>
      <p v-else-if="labs.length === 0" class="admin-labs__status">No labs yet.</p>

      <div v-else class="admin-labs__list">
        <article v-for="lab in labs" :key="lab.id" class="admin-labs__row">
          <div class="admin-labs__row-info">
            <div class="admin-labs__row-name-row">
              <span class="admin-labs__row-name">{{ lab.name }}</span>
              <StatusBadge :label="labPhaseLabel(labPhase(lab))" :tone="labPhase(lab)" />
            </div>
            <span class="admin-labs__row-id">{{ lab.id }}</span>
          </div>

          <div class="admin-labs__row-actions">
            <button type="button" class="button button--secondary" @click="openEdit(lab.id)">Edit</button>
            <RouterLink
              class="button button--secondary"
              :to="{ name: 'admin-queue', params: { labID: lab.id } }"
            >Queue</RouterLink>
            <button
              type="button"
              class="button button--secondary"
              :disabled="resetting === lab.id"
              @click="resetQuota(lab.id, lab.name)"
            >{{ resetting === lab.id ? 'Resetting…' : 'Reset Quota' }}</button>
          </div>
        </article>
      </div>
    </div>

    <p v-if="resetError" class="admin-labs__status admin-labs__status--error">{{ resetError }}</p>

    <LabEditDrawer
      :lab-id="drawerLabId"
      :open="drawerOpen"
      @close="onDrawerClose"
      @saved="onDrawerSaved"
    />
  </AdminShell>
</template>

<style scoped>
.admin-labs {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.admin-labs__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.admin-labs__title {
  font-size: 1.4rem;
  font-weight: 700;
  margin: 0;
}

.admin-labs__status {
  margin: 0;
  color: var(--text-secondary);
}

.admin-labs__status--error {
  color: var(--danger);
}

.admin-labs__list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.admin-labs__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: 10px;
  transition: border-color 150ms ease;
}

.admin-labs__row:hover {
  border-color: var(--border-strong);
}

.admin-labs__row-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.admin-labs__row-name-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.admin-labs__row-name {
  font-size: 0.95rem;
  font-weight: 600;
}

.admin-labs__row-id {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
}

.admin-labs__row-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-shrink: 0;
}
</style>
