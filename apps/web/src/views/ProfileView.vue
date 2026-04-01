<script setup lang="ts">
import { onMounted, ref } from 'vue';

type Key = {
  id: number;
  public_key: string;
  device_name: string;
  created_at: string;
};

const keys = ref<Key[]>([]);
const loading = ref(true);
const error = ref<string | null>(null);

function formatCreatedAt(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat('en-US', {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(date);
}

async function loadKeys() {
  loading.value = true;
  error.value = null;
  try {
    const response = await fetch('/api/keys', { credentials: 'include' });
    if (!response.ok) {
      throw new Error(`Failed to load keys: ${response.status}`);
    }
    const payload = (await response.json()) as { keys?: Key[] };
    keys.value = payload.keys ?? [];
  } catch (requestError) {
    error.value = requestError instanceof Error ? requestError.message : 'Failed to load keys.';
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  void loadKeys();
});
</script>

<template>
  <main class="page-shell profile-view" data-testid="page-shell">
    <section class="profile-view__panel">
      <div class="profile-view__header">
        <h1>Devices</h1>
        <p>Registered keys</p>
      </div>
      <p v-if="loading" class="profile-view__status">Loading keys…</p>
      <p v-else-if="error" class="profile-view__status">{{ error }}</p>
      <p v-else-if="keys.length === 0" class="profile-view__status">No keys are registered yet.</p>
      <ul v-else class="profile-view__list">
        <li v-for="key in keys" :key="key.id" class="profile-view__card">
          <div class="profile-view__card-head">
            <h2>{{ key.device_name }}</h2>
            <span>Key {{ key.id }}</span>
          </div>
          <p class="profile-view__mono">{{ key.public_key }}</p>
          <p class="profile-view__meta">Created {{ formatCreatedAt(key.created_at) }}</p>
        </li>
      </ul>
    </section>
  </main>
</template>

<style scoped>
.profile-view {
  padding-top: 24px;
}

.profile-view__panel {
  display: grid;
  gap: 18px;
  padding: 24px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
}

.profile-view__header {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 16px;
}

.profile-view__header h1,
.profile-view__header p {
  margin: 0;
}

.profile-view__header h1 {
  font-family: var(--font-mono);
  font-size: 1.7rem;
  font-weight: 700;
  letter-spacing: -0.04em;
}

.profile-view__header p {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.profile-view__status {
  margin: 0;
  color: var(--text-secondary);
}

.profile-view__list {
  display: grid;
  gap: 14px;
  padding: 0;
  margin: 0;
  list-style: none;
}

.profile-view__card {
  display: grid;
  gap: 10px;
  padding: 16px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-elevated);
}

.profile-view__card-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: baseline;
}

.profile-view__card-head h2,
.profile-view__card-head span,
.profile-view__mono,
.profile-view__meta {
  margin: 0;
}

.profile-view__card-head span,
.profile-view__meta {
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.profile-view__mono {
  overflow-wrap: anywhere;
  font-family: var(--font-mono);
  color: var(--accent);
}

@media (max-width: 767px) {
  .profile-view {
    padding-top: 8px;
  }

  .profile-view__header {
    flex-direction: column;
    align-items: start;
  }
}
</style>
