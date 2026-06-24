<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { DEFAULT_GRADE_LAB_ID, getMyGrade, type FinalGrade } from '../lib/grade';
import { login } from '../lib/session';
import PageTitleBlock from '../components/chrome/PageTitleBlock.vue';
import SectionHeader from '../components/chrome/SectionHeader.vue';

const props = defineProps<{ labId?: string }>();
const route = useRoute();

const labId = computed(() => {
  if (props.labId && props.labId.trim()) {
    return props.labId.trim();
  }
  const param = route.params.labID;
  if (typeof param === 'string' && param.trim()) {
    return param.trim();
  }
  const query = route.query.lab;
  if (typeof query === 'string' && query.trim()) {
    return query.trim();
  }
  return DEFAULT_GRADE_LAB_ID;
});

type ViewState = 'loading' | 'ok' | 'unpublished' | 'unauthorized' | 'error';
const state = ref<ViewState>('loading');
const grade = ref<FinalGrade | null>(null);
const errorMessage = ref('');

const breakdown = computed(() => {
  const g = grade.value;
  if (!g) return [];
  return [
    { label: '赛道', value: g.track && g.track.trim() ? g.track : '—' },
    { label: '赛道倍率 r', value: formatNumber(g.ratio) },
    { label: '性能分(85%)', value: formatNumber(g.perf_score) },
    { label: '赛道内百分位 p', value: formatNumber(g.percentile) },
    { label: '打榜分(15%)', value: formatNumber(g.board_score) }
  ];
});

function formatNumber(value: number | undefined, digits = 2) {
  if (value === undefined || value === null || Number.isNaN(Number(value))) {
    return '—';
  }
  return Number(value).toFixed(digits);
}

function formatUpdatedAt(value: string | undefined) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(date);
}

async function load() {
  state.value = 'loading';
  errorMessage.value = '';
  const result = await getMyGrade(labId.value);
  switch (result.status) {
    case 'ok':
      grade.value = result.grade;
      state.value = 'ok';
      break;
    case 'unpublished':
      grade.value = null;
      state.value = 'unpublished';
      break;
    case 'unauthorized':
      grade.value = null;
      state.value = 'unauthorized';
      break;
    case 'error':
      errorMessage.value = result.message;
      state.value = 'error';
      break;
  }
}

function goLogin() {
  login(route.fullPath);
}

onMounted(() => {
  void load();
});

watch(labId, () => {
  void load();
});
</script>

<template>
  <main class="page-shell grade-view" data-testid="grade-view">
    <PageTitleBlock
      title="课程成绩"
      eyebrow="Final grade"
      :lede="`Lab ${labId} · 总评由助教在外部计算后导入，此处只读展示。`"
    />

    <section v-if="state === 'loading'" class="grade-view__panel">
      <p class="grade-view__status">正在加载成绩…</p>
    </section>

    <section v-else-if="state === 'unauthorized'" class="grade-view__panel">
      <SectionHeader title="请先登录" subtitle="Sign in" />
      <p class="grade-view__status">登录后即可查看课程成绩。</p>
      <button class="grade-view__button" type="button" @click="goLogin">登录</button>
    </section>

    <section v-else-if="state === 'unpublished'" class="grade-view__panel">
      <SectionHeader title="成绩尚未发布" subtitle="Not published" />
      <p class="grade-view__status">
        当前 Lab（<code>{{ labId }}</code>）的成绩尚未发布，请稍后再来查看。
      </p>
    </section>

    <section v-else-if="state === 'error'" class="grade-view__panel">
      <SectionHeader title="加载失败" subtitle="Error" />
      <p class="grade-view__status">{{ errorMessage }}</p>
      <button class="grade-view__button" type="button" @click="load">重试</button>
    </section>

    <template v-else-if="state === 'ok' && grade">
      <section class="grade-view__panel grade-view__total">
        <SectionHeader title="总评" subtitle="Total score" />
        <p class="grade-view__total-value" data-testid="grade-total">{{ formatNumber(grade.total) }}</p>
        <p class="grade-view__formula">总评 = 0.85 × 性能分 + 0.15 × 打榜分</p>
      </section>

      <section class="grade-view__panel">
        <SectionHeader title="分项" subtitle="Breakdown" />
        <dl class="grade-view__grid">
          <div v-for="item in breakdown" :key="item.label" class="grade-view__cell">
            <dt>{{ item.label }}</dt>
            <dd>{{ item.value }}</dd>
          </div>
        </dl>
      </section>

      <section v-if="grade.remark && grade.remark.trim()" class="grade-view__panel">
        <SectionHeader title="备注" subtitle="Remark" />
        <p class="grade-view__remark">{{ grade.remark }}</p>
      </section>

      <p v-if="grade.updated_at" class="grade-view__meta">
        更新于 {{ formatUpdatedAt(grade.updated_at) }}
      </p>
    </template>
  </main>
</template>

<style scoped>
.grade-view {
  padding-top: 24px;
  display: grid;
  gap: 16px;
}

.grade-view__panel {
  display: grid;
  gap: 14px;
  padding: 24px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
}

.grade-view__status {
  margin: 0;
  color: var(--text-secondary);
  line-height: 1.55;
}

.grade-view__status code {
  font-family: var(--font-mono);
}

.grade-view__button {
  justify-self: start;
  min-height: 44px;
  padding: 10px 18px;
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

.grade-view__total {
  text-align: center;
}

.grade-view__total-value {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 3rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  color: var(--text-primary);
}

.grade-view__formula {
  margin: 0;
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.8rem;
}

.grade-view__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
  margin: 0;
}

.grade-view__cell {
  display: grid;
  gap: 6px;
  padding: 14px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-elevated);
}

.grade-view__cell dt {
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.7rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.grade-view__cell dd {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--text-primary);
}

.grade-view__remark {
  margin: 0;
  color: var(--text-secondary);
  line-height: 1.6;
  white-space: pre-wrap;
}

.grade-view__meta {
  margin: 0;
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.74rem;
}
</style>
