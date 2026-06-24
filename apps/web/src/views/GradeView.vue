<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { getMyGrade, type FinalGrade } from '../lib/grade';
import { login } from '../lib/session';
import PageTitleBlock from '../components/chrome/PageTitleBlock.vue';
import SectionHeader from '../components/chrome/SectionHeader.vue';
import GradeDetail from '../components/grade/GradeDetail.vue';

const props = defineProps<{ labId?: string }>();
const route = useRoute();

const labId = computed(() => {
  if (props.labId && props.labId.trim()) {
    return props.labId.trim();
  }
  const param = route.params.labID;
  return typeof param === 'string' ? param.trim() : '';
});

type ViewState = 'loading' | 'ok' | 'unpublished' | 'unauthorized' | 'error';
const state = ref<ViewState>('loading');
const grade = ref<FinalGrade | null>(null);
const errorMessage = ref('');

async function load() {
  if (!labId.value) {
    errorMessage.value = '缺少 Lab 标识。';
    state.value = 'error';
    return;
  }
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
      :lede="`Lab ${labId} · 成绩由助教导入，此处仅供查询。`"
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

    <GradeDetail v-else-if="state === 'ok' && grade" :grade="grade" />
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
</style>
