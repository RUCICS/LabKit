<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { RouterLink } from 'vue-router';
import { readAPIError } from '../lib/http';
import PageTitleBlock from '../components/chrome/PageTitleBlock.vue';
import SectionHeader from '../components/chrome/SectionHeader.vue';
import DeviceKeyRow from '../components/profile/DeviceKeyRow.vue';

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
    <PageTitleBlock
      title="Profile"
      eyebrow="Personal console"
      lede="Update your identity, manage device keys, and review recent activity."
    />

    <section class="profile-view__panel">
      <SectionHeader title="Identity" subtitle="Global profile" />
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
      <SectionHeader title="Devices" subtitle="Registered keys" />
      <p v-if="loading" class="profile-view__status">Loading keys…</p>
      <p v-else-if="error" class="profile-view__status">{{ error }}</p>
      <p v-else-if="keys.length === 0" class="profile-view__status">No keys are registered yet.</p>
      <ul v-else class="profile-view__rows">
        <li v-for="key in keys" :key="key.id">
          <DeviceKeyRow :device-key="key" :created-at="formatCreatedAt(key.created_at)" />
        </li>
      </ul>
    </section>

    <section class="profile-view__panel">
      <SectionHeader title="Activity" subtitle="Recent submissions" />
      <p v-if="error" class="profile-view__status">{{ error }}</p>
      <p v-else-if="(profile?.recent_activity?.length ?? 0) === 0" class="profile-view__status">
        No recent activity yet.
      </p>
      <ul v-else class="profile-view__rows">
        <li v-for="item in profile?.recent_activity" :key="item.id">
          <div class="profile-view__row">
            <div class="profile-view__row-copy">
              <p class="profile-view__row-title">{{ item.lab_id }}</p>
              <p class="profile-view__row-subtitle">
                Submitted {{ formatCreatedAt(item.created_at) }}
              </p>
            </div>

            <div class="profile-view__row-meta">
              <span class="profile-view__row-tag">{{ item.status }}</span>
              <span class="profile-view__row-actions">
                <RouterLink class="profile-view__row-action" :to="`/labs/${item.lab_id}/board`">
                  Board
                </RouterLink>
                <RouterLink class="profile-view__row-action" :to="`/labs/${item.lab_id}/history`">
                  History
                </RouterLink>
              </span>
            </div>
          </div>
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
  gap: 16px;
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

.profile-view__status {
  margin: 0;
  color: var(--text-secondary);
}

.profile-view__rows {
  display: grid;
  gap: 10px;
  padding: 0;
  margin: 0;
  list-style: none;
}

.profile-view__row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 14px;
  min-height: 44px;
  padding: 12px 14px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-elevated);
}

.profile-view__row-copy {
  min-width: 0;
  display: grid;
  gap: 4px;
  flex: 1 1 14rem;
}

.profile-view__row-title,
.profile-view__row-subtitle {
  margin: 0;
}

.profile-view__row-title {
  min-width: 0;
  font-family: var(--font-mono);
  font-size: 0.8rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.profile-view__row-subtitle {
  min-width: 0;
  color: var(--text-secondary);
  font-size: 0.9rem;
  line-height: 1.55;
}

.profile-view__row-meta {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  flex: 0 1 auto;
}

.profile-view__row-tag {
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.profile-view__row-actions {
  display: flex;
  gap: 10px;
  align-items: center;
}

.profile-view__row-action {
  padding: 8px 10px;
  border-radius: 999px;
  border: 1px solid var(--border-default);
  background: transparent;
  color: var(--text-tertiary);
  text-decoration: none;
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.profile-view__row-action:hover {
  color: var(--text-primary);
  border-color: color-mix(in srgb, var(--accent) 30%, var(--border-default));
}

@media (max-width: 767px) {
  .profile-view {
    padding-top: 8px;
  }
}
</style>
