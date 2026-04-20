<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { RouterLink } from 'vue-router';
import { readAPIError } from '../lib/http';

type Key = {
  id: number;
  public_key: string;
  device_name: string;
  created_at: string;
};

type RecentSubmission = {
  id: string;
  lab_id: string;
  status: string;
  created_at: string;
};

type UserProfile = {
  user_id: number;
  student_id: string;
  nickname: string;
  recent_activity?: RecentSubmission[];
};

const profile = ref<UserProfile | null>(null);
const nicknameDraft = ref('');
const saving = ref(false);

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

async function loadProfile() {
  error.value = null;
  try {
    const response = await fetch('/api/profile', { credentials: 'include' });
    if (!response.ok) {
      throw new Error(await readAPIError(response, 'Failed to load profile.'));
    }
    const payload = (await response.json()) as UserProfile;
    profile.value = payload;
    nicknameDraft.value = payload.nickname ?? '';
  } catch (requestError) {
    error.value = requestError instanceof Error ? requestError.message : 'Failed to load profile.';
  }
}

async function loadKeys() {
  loading.value = true;
  error.value = null;
  try {
    const response = await fetch('/api/keys', { credentials: 'include' });
    if (!response.ok) {
      throw new Error(await readAPIError(response, `Failed to load keys: ${response.status}`));
    }
    const payload = (await response.json()) as { keys?: Key[] };
    keys.value = payload.keys ?? [];
  } catch (requestError) {
    error.value = requestError instanceof Error ? requestError.message : 'Failed to load keys.';
  } finally {
    loading.value = false;
  }
}

async function saveNickname() {
  if (saving.value) return;
  saving.value = true;
  error.value = null;
  try {
    const response = await fetch('/api/profile', {
      method: 'PUT',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ nickname: nicknameDraft.value })
    });
    if (!response.ok) {
      throw new Error(await readAPIError(response, 'Failed to update nickname.'));
    }
    profile.value = (await response.json()) as UserProfile;
    nicknameDraft.value = profile.value.nickname ?? nicknameDraft.value;
  } catch (requestError) {
    error.value = requestError instanceof Error ? requestError.message : 'Failed to update nickname.';
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  void loadProfile();
  void loadKeys();
});
</script>

<template>
  <main class="page-shell profile-view" data-testid="page-shell">
    <section class="profile-view__panel">
      <div class="profile-view__header">
        <h1>Identity</h1>
        <p>Global profile</p>
      </div>
      <form class="profile-view__form" @submit.prevent="saveNickname">
        <label class="profile-view__field">
          <span>Nickname</span>
          <input v-model="nicknameDraft" class="profile-view__input" autocomplete="nickname" />
        </label>
        <button class="profile-view__button" type="submit" :disabled="saving">
          {{ saving ? 'Saving…' : 'Save' }}
        </button>
      </form>
    </section>

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

    <section class="profile-view__panel">
      <div class="profile-view__header">
        <h1>Activity</h1>
        <p>Recent submissions</p>
      </div>
      <p v-if="error" class="profile-view__status">{{ error }}</p>
      <p v-else-if="(profile?.recent_activity?.length ?? 0) === 0" class="profile-view__status">
        No recent activity yet.
      </p>
      <ul v-else class="profile-view__list">
        <li v-for="item in profile?.recent_activity" :key="item.id" class="profile-view__card">
          <div class="profile-view__card-head">
            <h2>{{ item.lab_id }}</h2>
            <span>{{ item.status }}</span>
          </div>
          <p class="profile-view__meta">Submitted {{ formatCreatedAt(item.created_at) }}</p>
          <p class="profile-view__links">
            <RouterLink class="profile-view__link" :to="`/labs/${item.lab_id}/board`">Board</RouterLink>
            <RouterLink class="profile-view__link" :to="`/labs/${item.lab_id}/history`">History</RouterLink>
          </p>
        </li>
      </ul>
    </section>
  </main>
</template>

<style scoped>
.profile-view {
  padding-top: 24px;
  display: grid;
  gap: 16px;
}

.profile-view__panel {
  display: grid;
  gap: 18px;
  padding: 24px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
}

.profile-view__form {
  display: flex;
  align-items: end;
  gap: 12px;
  flex-wrap: wrap;
}

.profile-view__field {
  display: grid;
  gap: 6px;
  flex: 1 1 280px;
}

.profile-view__field span {
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.profile-view__input {
  min-height: 44px;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid var(--border-default);
  background: var(--bg-elevated);
  color: var(--text-primary);
}

.profile-view__button {
  min-height: 44px;
  padding: 10px 16px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--accent) 35%, var(--border-default));
  background: color-mix(in srgb, var(--accent) 16%, var(--bg-surface));
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  cursor: pointer;
}

.profile-view__button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
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

.profile-view__links {
  display: flex;
  gap: 12px;
}

.profile-view__link {
  color: var(--text-secondary);
  text-decoration: none;
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.profile-view__link:hover {
  color: var(--text-primary);
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
