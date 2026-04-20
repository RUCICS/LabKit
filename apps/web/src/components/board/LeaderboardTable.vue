<script setup lang="ts">
import { computed } from 'vue';
import type { LeaderboardMetric, LeaderboardRow } from './types';
import { metricTone } from '../../lib/labs';

const props = defineProps<{
  rows: LeaderboardRow[];
  metrics: LeaderboardMetric[];
  selectedMetricId: string;
  closeAt?: string;
  apiHint: string;
  metricUnits?: Record<string, string>;
}>();

const lastUpdated = computed(() => {
  const values = props.rows
    .map((row) => Date.parse(row.updated_at))
    .filter((value) => !Number.isNaN(value))
    .sort((left, right) => right - left);
  return values[0] ?? null;
});

const footerSummary = computed(() => {
  const parts = [`${props.rows.length} on board`];
  if (lastUpdated.value !== null) {
    parts.push(`Last update ${formatFooterTime(lastUpdated.value)}`);
  }
  const close = Date.parse(props.closeAt ?? '');
  if (!Number.isNaN(close)) {
    parts.push(`Closes ${formatFooterDate(close)}`);
  }
  return parts.join(' · ');
});

function scoreForRow(row: LeaderboardRow, metricId: string) {
  return row.scores.find((score) => score.metric_id === metricId)?.value;
}

function formatScore(value: number | undefined) {
  if (typeof value !== 'number') {
    return '—';
  }
  return new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(value);
}

function metricUnit(metricId: string) {
  return props.metricUnits?.[metricId] ?? '';
}

function formatUpdatedAt(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }
  return new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit'
  }).format(date);
}

function formatFooterTime(value: number) {
  return new Intl.DateTimeFormat('en-US', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(value);
}

function formatFooterDate(value: number) {
  return new Intl.DateTimeFormat('en-US', {
    month: '2-digit',
    day: '2-digit'
  }).format(value);
}

function trackClass(track: string | undefined) {
  return `board-table__track-indicator--${metricTone(track ?? '')}`;
}
</script>

<template>
  <div class="board-table">
    <table>
      <thead>
        <tr>
          <th scope="col">Rank</th>
          <th scope="col">Nickname</th>
          <th
            v-for="metric in metrics"
            :key="metric.id"
            scope="col"
            :class="{ 'board-table__metric--selected': metric.id === selectedMetricId }"
          >
            {{ metric.name }}{{ metric.id === selectedMetricId ? (metric.sort === 'asc' ? ' ↑' : ' ↓') : '' }}
          </th>
          <th scope="col">Updated</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in rows"
          :key="`${row.rank}-${row.nickname}`"
          data-testid="leaderboard-row"
          :class="{ 'board-table__row--current': row.current_user }"
          :style="{ '--row-delay': `${Math.max(row.rank - 1, 0) * 30}ms` }"
        >
          <td class="board-table__rank">{{ row.rank }}</td>
          <td class="board-table__nickname">
            <span
              v-if="row.track"
              class="board-table__track-indicator"
              :class="trackClass(row.track)"
            />
            <span>{{ row.nickname }}</span>
          </td>
          <td
            v-for="metric in metrics"
            :key="metric.id"
            class="board-table__metric"
            :class="{ 'board-table__metric-cell--selected': metric.id === selectedMetricId }"
          >
            <span>{{ formatScore(scoreForRow(row, metric.id)) }}</span>
            <span v-if="metricUnit(metric.id)" class="board-table__unit">{{ metricUnit(metric.id) }}</span>
          </td>
          <td class="board-table__updated">{{ formatUpdatedAt(row.updated_at) }}</td>
        </tr>
      </tbody>
      <tfoot>
        <tr>
          <td :colspan="metrics.length + 3">
            <div class="board-table__footer">
              <span>{{ footerSummary }}</span>
              <span class="board-table__api">{{ apiHint }}</span>
            </div>
          </td>
        </tr>
      </tfoot>
    </table>
  </div>
</template>

<style scoped>
.board-table {
  overflow: auto;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
}

table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
}

th,
td {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-subtle);
  text-align: left;
  transition: background 150ms ease;
}

th {
  position: sticky;
  top: 0;
  z-index: 1;
  background: var(--bg-elevated);
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.1em;
}

.board-table__metric--selected {
  color: var(--accent);
}

tbody tr:last-child td {
  border-bottom: 0;
}

tbody tr {
  animation: board-row-enter 300ms ease-out both;
  animation-delay: var(--row-delay, 0ms);
}

tbody tr:hover td {
  background: var(--bg-hover);
}

.board-table__row--current td {
  background: var(--accent-dim);
}

.board-table__row--current td:first-child {
  box-shadow: inset 2px 0 0 var(--accent);
}

.board-table__rank,
.board-table__metric,
.board-table__updated {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}

.board-table__rank {
  width: 64px;
  text-align: center;
  color: var(--text-tertiary);
  font-weight: 700;
}

.board-table tbody tr:nth-child(1) .board-table__rank {
  color: var(--rank-1);
  text-shadow: 0 0 14px rgba(251, 191, 36, 0.2);
}

.board-table tbody tr:nth-child(2) .board-table__rank {
  color: var(--rank-2);
}

.board-table tbody tr:nth-child(3) .board-table__rank {
  color: var(--rank-3);
}

.board-table__nickname {
  display: flex;
  align-items: center;
  gap: 10px;
  font-weight: 500;
}

.board-table__track-indicator {
  width: 4px;
  height: 20px;
  border-radius: 2px;
  flex: 0 0 auto;
}

.board-table__track-indicator--throughput {
  background: var(--track-throughput);
}

.board-table__track-indicator--latency {
  background: var(--track-latency);
}

.board-table__track-indicator--fairness {
  background: var(--track-fairness);
}

.board-table__metric {
  width: 140px;
  text-align: right;
  font-size: 0.9rem;
  font-weight: 400;
  color: var(--text-secondary);
}

.board-table__updated {
  text-align: right;
}

.board-table__unit {
  margin-left: 1px;
  color: var(--text-tertiary);
  font-size: 0.7rem;
  font-weight: 400;
}

.board-table__updated {
  width: 130px;
  color: var(--text-secondary);
  font-size: 0.78rem;
}

.board-table__metric-cell--selected {
  color: var(--text-primary);
  font-size: 0.95rem;
  font-weight: 600;
}

tfoot tr td {
  padding: 0;
}

tfoot td {
  background: var(--bg-elevated);
  border-top: 1px solid var(--border-default);
  border-bottom: 0;
}

.board-table__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 16px;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.04em;
}

.board-table__api {
  color: var(--text-tertiary);
  transition: color 150ms ease;
}

.board-table__api:hover {
  color: var(--text-secondary);
}

@media (max-width: 767px) {
  .board-table__updated,
  th:last-child {
    display: none;
  }

  .board-table__metric {
    width: 100px;
  }

  .board-table__footer {
    flex-direction: column;
    align-items: flex-start;
  }
}

@keyframes board-row-enter {
  from {
    opacity: 0;
    transform: translateY(8px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
