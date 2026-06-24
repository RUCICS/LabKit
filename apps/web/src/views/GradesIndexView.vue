<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { getMyGrades, type FinalGrade } from '../lib/grade';
import { login } from '../lib/session';
import PageTitleBlock from '../components/chrome/PageTitleBlock.vue';
import SectionHeader from '../components/chrome/SectionHeader.vue';
import GradeDetail from '../components/grade/GradeDetail.vue';

const route = useRoute();

type ViewState = 'loading' | 'ok' | 'unauthorized' | 'error';
const state = ref<ViewState>('loading');
const grades = ref<FinalGrade[]>([]);
const errorMessage = ref('');

async function load() {
  state.value = 'loading';
  errorMessage.value = '';
  const result = await getMyGrades();
  switch (result.status) {
    case 'ok':
      grades.value = result.grades;
      state.value = 'ok';
      break;
    case 'unauthorized':
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
</script>

<template>
  <main class="page-shell grades-index" data-testid="grades-index">
    <PageTitleBlock title="我的成绩" eyebrow="Final grades" lede="你在各 Lab 的已发布成绩。" />

    <section v-if="state === 'loading'" class="grades-index__panel">
      <p class="grades-index__status">正在加载成绩…</p>
    </section>

    <section v-else-if="state === 'unauthorized'" class="grades-index__panel">
      <SectionHeader title="请先登录" subtitle="Sign in" />
      <p class="grades-index__status">登录后即可查看课程成绩。</p>
      <button class="grades-index__button" type="button" @click="goLogin">登录</button>
    </section>

    <section v-else-if="state === 'error'" class="grades-index__panel">
      <SectionHeader title="加载失败" subtitle="Error" />
      <p class="grades-index__status">{{ errorMessage }}</p>
      <button class="grades-index__button" type="button" @click="load">重试</button>
    </section>

    <section v-else-if="grades.length === 0" class="grades-index__panel">
      <p class="grades-index__status" data-testid="grades-empty">暂无已发布的成绩。</p>
    </section>

    <template v-else>
      <article v-for="g in grades" :key="g.lab_id" class="grades-index__lab" data-testid="grades-index-lab">
        <RouterLink class="grades-index__lab-id" :to="`/labs/${g.lab_id}/grade`">{{ g.lab_id }}</RouterLink>
        <GradeDetail :grade="g" />
      </article>
    </template>
  </main>
</template>

<style scoped>
.grades-index {
  padding-top: 24px;
  display: grid;
  gap: 16px;
}

.grades-index__panel {
  display: grid;
  gap: 14px;
  padding: 24px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
}

.grades-index__status {
  margin: 0;
  color: var(--text-secondary);
  line-height: 1.55;
}

.grades-index__button {
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

.grades-index__lab {
  display: grid;
  gap: 12px;
}

.grades-index__lab-id {
  justify-self: start;
  font-family: var(--font-mono);
  font-size: 0.78rem;
  letter-spacing: 0.04em;
  color: var(--text-tertiary);
  text-decoration: none;
}

.grades-index__lab-id:hover {
  color: var(--text-primary);
}
</style>
