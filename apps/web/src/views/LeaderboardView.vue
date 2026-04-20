<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import LeaderboardEmptyState from '../components/board/LeaderboardEmptyState.vue';
import LeaderboardHiddenState from '../components/board/LeaderboardHiddenState.vue';
import LeaderboardMetricTabs from '../components/board/LeaderboardMetricTabs.vue';
import LeaderboardTable from '../components/board/LeaderboardTable.vue';
import QuotaSummaryBar from '../components/chrome/QuotaSummaryBar.vue';
import type { LeaderboardBoard, LeaderboardLabDetail } from '../components/board/types';
import { readAPIError } from '../lib/http';
import { metricAccentTokens } from '../lib/labs';

const props = defineProps<{
  labId: string;
}>();

const board = ref<LeaderboardBoard | null>(null);
const lab = ref<LeaderboardLabDetail | null>(null);
const loading = ref(true);
const hidden = ref(false);
const error = ref<string | null>(null);
const activeMetric = ref('');
const profileNickname = ref('');
const profileTrack = ref('');
const profileBusy = ref<'nickname' | 'track' | ''>('');
const profileNotice = ref('');
const profileError = ref('');
let requestSeq = 0;

const showTabs = computed(() => (board.value?.metrics.length ?? 0) > 1);
const selectedMetric = computed(
  () => board.value?.metrics.find((metric) => metric.id === activeMetric.value) ?? null
);
const currentUserRow = computed(() => board.value?.rows.find((row) => row.current_user) ?? null);
const labTitle = computed(() => lab.value?.name?.trim() || formatLabTitle(props.labId));
const canPickTrack = computed(() => Boolean(lab.value?.manifest?.board?.pick));
const metricUnits = computed(() =>
  Object.fromEntries((lab.value?.manifest?.metrics ?? []).map((metric) => [metric.id, metric.unit ?? '']))
);
const trackOptions = computed(() => lab.value?.manifest?.metrics ?? []);
const statItems = computed(() => {
  const closeAt = Date.parse(lab.value?.manifest?.schedule?.close ?? '');
  return [
    {
      label: 'Participants',
      value: String(board.value?.rows.length ?? 0)
    },
    {
      label: 'Metrics',
      value: String(board.value?.metrics.length ?? 0)
    },
    {
      label: 'Remaining',
      value: Number.isNaN(closeAt) ? '—' : formatRemaining(closeAt)
    }
  ];
});
const accentStyle = computed(() => {
  const metricIndex = board.value?.metrics.findIndex((m) => m.id === activeMetric.value) ?? -1;
  const token = metricAccentTokens(activeMetric.value || props.labId, metricIndex >= 0 ? metricIndex : undefined);
  return {
    '--accent': `var(${token.color})`,
    '--accent-dim': `var(${token.dim})`
  };
});

function metricIdFromLocation() {
  return new URLSearchParams(window.location.search).get('by')?.trim() ?? '';
}

function setMetricInLocation(metricId: string) {
  const url = new URL(window.location.href);
  if (metricId) {
    url.searchParams.set('by', metricId);
  } else {
    url.searchParams.delete('by');
  }
  window.history.replaceState({}, '', `${url.pathname}${url.search}${url.hash}`);
}

function formatLabTitle(labId: string) {
  const words = labId
    .replace(/[-_]+/g, ' ')
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1));
  return `${words.join(' ')} Lab`;
}

function formatRemaining(closeAt: number) {
  const remaining = Math.max(closeAt - Date.now(), 0);
  const days = Math.ceil(remaining / (1000 * 60 * 60 * 24));
  return `${days}d`;
}

async function loadBoard(metricId = metricIdFromLocation()) {
  const requestId = ++requestSeq;
  loading.value = true;
  hidden.value = false;
  error.value = null;
  try {
    const query = metricId ? `?by=${encodeURIComponent(metricId)}` : '';
    const [boardResponse, labResponse] = await Promise.all([
      fetch(`/api/labs/${encodeURIComponent(props.labId)}/board${query}`, { credentials: 'include' }),
      fetch(`/api/labs/${encodeURIComponent(props.labId)}`)
    ]);
    if (requestId !== requestSeq) {
      return;
    }
    if (boardResponse.status === 404) {
      const apiError = await readBoardError(boardResponse);
      if (requestId !== requestSeq) {
        return;
      }
      if (apiError?.code === 'lab_hidden') {
        board.value = null;
        hidden.value = true;
        return;
      }
      throw new Error(apiError?.message ?? 'Lab not found');
    }
    if (!boardResponse.ok) {
      throw new Error(await readAPIError(boardResponse, 'Failed to load leaderboard'));
    }
    if (!labResponse.ok && labResponse.status !== 404) {
      throw new Error(await readAPIError(labResponse, 'Failed to load lab details'));
    }
    board.value = (await boardResponse.json()) as LeaderboardBoard;
    lab.value = labResponse.ok ? ((await labResponse.json()) as LeaderboardLabDetail) : null;
    activeMetric.value = board.value.selected_metric;
    if (currentUserRow.value) {
      profileNickname.value = currentUserRow.value.nickname;
      profileTrack.value = currentUserRow.value.track ?? board.value.selected_metric;
    } else if (profileTrack.value === '') {
      profileTrack.value = board.value.selected_metric;
    }
    setMetricInLocation(board.value.selected_metric);
  } catch (requestError) {
    if (requestId !== requestSeq) {
      return;
    }
    error.value =
      requestError instanceof Error ? requestError.message : 'Failed to load leaderboard';
  } finally {
    if (requestId === requestSeq) {
      loading.value = false;
    }
  }
}

async function readBoardError(response: Response) {
  try {
    const payload = (await response.json()) as {
      error?: { code?: string; message?: string };
    };
    return payload.error ?? null;
  } catch {
    return null;
  }
}

async function saveNickname() {
  profileBusy.value = 'nickname';
  profileError.value = '';
  profileNotice.value = '';
  try {
    const response = await fetch(`/api/labs/${encodeURIComponent(props.labId)}/nickname`, {
      method: 'PUT',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ nickname: profileNickname.value.trim() })
    });
    if (!response.ok) {
      throw new Error(await readAPIError(response, 'Failed to update nickname'));
    }
    profileNotice.value = 'Profile updated';
    await loadBoard(activeMetric.value);
  } catch (requestError) {
    profileError.value =
      requestError instanceof Error ? requestError.message : 'Failed to update nickname';
  } finally {
    profileBusy.value = '';
  }
}

async function saveTrack() {
  profileBusy.value = 'track';
  profileError.value = '';
  profileNotice.value = '';
  try {
    const response = await fetch(`/api/labs/${encodeURIComponent(props.labId)}/track`, {
      method: 'PUT',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ track: profileTrack.value })
    });
    if (!response.ok) {
      throw new Error(await readAPIError(response, 'Failed to update track'));
    }
    profileNotice.value = 'Profile updated';
    await loadBoard(activeMetric.value);
  } catch (requestError) {
    profileError.value =
      requestError instanceof Error ? requestError.message : 'Failed to update track';
  } finally {
    profileBusy.value = '';
  }
}

function handleMetricSelect(metricId: string) {
  if (metricId === activeMetric.value) {
    return;
  }
  setMetricInLocation(metricId);
  void loadBoard(metricId);
}

watch(
  () => props.labId,
  () => {
    void loadBoard();
  }
);

onMounted(() => {
  void loadBoard();
});
</script>

<template>
  <main class="page-shell leaderboard-view" :style="accentStyle">
    <section class="leaderboard-view__hero" v-if="board">
      <div class="leaderboard-view__title-block">
        <h1 class="leaderboard-view__title">{{ labTitle }}</h1>
        <p class="leaderboard-view__subtitle">{{ board.lab_id }}</p>
      </div>
      <div class="leaderboard-view__stats">
        <article
          v-for="item in statItems"
          :key="item.label"
          class="leaderboard-view__stat"
        >
          <strong>{{ item.value }}</strong>
          <span>{{ item.label }}</span>
        </article>
      </div>
    </section>

    <section class="leaderboard-view__content">
      <p v-if="loading" class="leaderboard-view__status">Loading leaderboard…</p>
      <p v-else-if="error" class="leaderboard-view__status">{{ error }}</p>
      <LeaderboardHiddenState v-else-if="hidden" />
      <template v-else-if="board">
        <div class="leaderboard-view__utility">
          <QuotaSummaryBar :quota="board.quota" />
          <a class="button button--secondary" :href="`/labs/${board.lab_id}/history`">My history</a>
        </div>

        <section
          v-if="board.quota || currentUserRow || canPickTrack"
          class="leaderboard-view__profile"
        >
          <div class="leaderboard-view__profile-copy">
            <h2>My board profile</h2>
            <p>Control how your row appears on this board.</p>
          </div>
          <form class="leaderboard-view__profile-form" @submit.prevent="saveNickname">
            <label class="field field--stacked">
              <span>Nickname</span>
              <input
                v-model="profileNickname"
                name="nickname"
                type="text"
                placeholder="Ada"
              />
            </label>
            <button type="submit" class="button" :disabled="profileBusy !== ''">
              {{ profileBusy === 'nickname' ? 'Saving…' : 'Save nickname' }}
            </button>
          </form>

          <form
            v-if="canPickTrack"
            class="leaderboard-view__profile-form"
            @submit.prevent="saveTrack"
          >
            <label class="field field--stacked">
              <span>Track</span>
              <select v-model="profileTrack" name="track">
                <option v-for="metric in trackOptions" :key="metric.id" :value="metric.id">
                  {{ metric.name }}
                </option>
              </select>
            </label>
            <button type="submit" class="button button--secondary" :disabled="profileBusy !== ''">
              {{ profileBusy === 'track' ? 'Saving…' : 'Save track' }}
            </button>
          </form>

          <p v-if="profileError" class="leaderboard-view__status">{{ profileError }}</p>
          <p v-else-if="profileNotice" class="leaderboard-view__notice">{{ profileNotice }}</p>
        </section>

        <LeaderboardMetricTabs
          v-if="showTabs"
          :metrics="board.metrics"
          :selected-metric-id="activeMetric"
          @select="handleMetricSelect"
        />
        <LeaderboardTable
          v-if="board.rows.length > 0 && selectedMetric"
          :rows="board.rows"
          :metrics="board.metrics"
          :selected-metric-id="activeMetric"
          :close-at="lab?.manifest?.schedule?.close"
          :api-hint="`GET /api/labs/${board.lab_id}/board`"
          :metric-units="metricUnits"
        />
        <LeaderboardEmptyState v-else />
      </template>
    </section>
  </main>
</template>

<style scoped>
.leaderboard-view {
  display: grid;
  gap: 24px;
}

.leaderboard-view__hero {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
}

.leaderboard-view__title-block,
.leaderboard-view__profile,
.leaderboard-view__profile-copy,
.leaderboard-view__profile-form,
.leaderboard-view__stat,
.leaderboard-view__content {
  display: grid;
}

.leaderboard-view__title-block {
  gap: 10px;
}

.leaderboard-view__title {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 1.7rem;
  font-weight: 700;
  letter-spacing: -0.04em;
  line-height: 1;
}

.leaderboard-view__subtitle {
  margin: 0;
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.leaderboard-view__stats {
  display: flex;
  align-items: flex-end;
  gap: 24px;
  flex-wrap: wrap;
}

.leaderboard-view__stat {
  gap: 6px;
}

.leaderboard-view__stat strong {
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 1.5rem;
  font-weight: 700;
  line-height: 1;
}

.leaderboard-view__stat span,
.leaderboard-view__status,
.leaderboard-view__notice {
  font-family: var(--font-mono);
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.leaderboard-view__content {
  gap: 12px;
}

.leaderboard-view__utility {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.leaderboard-view__profile {
  gap: 14px;
  padding: 18px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
}

.leaderboard-view__profile-copy {
  gap: 4px;
}

.leaderboard-view__profile-copy h2,
.leaderboard-view__profile-copy p {
  margin: 0;
}

.leaderboard-view__profile-copy h2 {
  font-family: var(--font-mono);
  font-size: 0.95rem;
}

.leaderboard-view__profile-copy p,
.leaderboard-view__status {
  color: var(--text-secondary);
}

.leaderboard-view__notice {
  margin: 0;
  color: var(--color-open);
}

.leaderboard-view__profile-form {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: end;
}

@media (max-width: 767px) {
  .leaderboard-view__hero,
  .leaderboard-view__utility,
  .leaderboard-view__profile-form {
    flex-direction: column;
    align-items: flex-start;
  }

  .leaderboard-view__stats {
    width: 100%;
    justify-content: space-between;
    gap: 16px;
  }

  .leaderboard-view__profile-form {
    grid-template-columns: 1fr;
    width: 100%;
  }
}
</style>
