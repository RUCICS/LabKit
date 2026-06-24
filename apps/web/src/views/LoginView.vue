<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { isAuthenticated, login, refreshSession, sessionUser } from '../lib/session';

const route = useRoute();
const router = useRouter();

const nextPath = computed(() => {
  const raw = route.query.next;
  const value = typeof raw === 'string' ? raw : '';
  return value.startsWith('/') && !value.startsWith('//') ? value : '/';
});

function signIn() {
  login(nextPath.value);
}

function goNext() {
  void router.push(nextPath.value);
}

onMounted(() => {
  void refreshSession();
});
</script>

<template>
  <main class="login-view" data-testid="login-view">
    <section class="login-card">
      <div class="login-card__brand">
        <span class="login-card__brand-mark" aria-hidden="true">L</span>
        <span class="login-card__brand-name">LabKit</span>
      </div>

      <div v-if="isAuthenticated" class="login-card__body">
        <h1 class="login-card__title">已登录</h1>
        <p class="login-card__subtitle">当前账号：{{ sessionUser?.student_id }}</p>
        <button class="login-card__button" type="button" @click="goNext">继续</button>
      </div>

      <div v-else class="login-card__body">
        <h1 class="login-card__title">登录 LabKit</h1>
        <p class="login-card__subtitle">请使用学校账号登录以继续。</p>
        <button class="login-card__button" type="button" data-testid="sign-in" @click="signIn">
          微人大登录
        </button>
      </div>
    </section>
  </main>
</template>

<style scoped>
.login-view {
  min-height: 60vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px 16px;
}

.login-card {
  width: 100%;
  max-width: 380px;
  display: grid;
  gap: 28px;
}

.login-card__brand {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.login-card__brand-mark {
  width: 34px;
  height: 34px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--accent);
  color: var(--text-inverse);
  font-family: var(--font-mono);
  font-weight: 700;
}

.login-card__brand-name {
  font-family: var(--font-mono);
  font-size: 1.25rem;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.login-card__body {
  display: grid;
  gap: 14px;
  padding: 32px 28px;
  border: 1px solid var(--border-default);
  border-radius: 12px;
  background: var(--bg-surface);
  text-align: center;
}

.login-card__title {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 700;
}

.login-card__subtitle {
  margin: 0;
  color: var(--text-secondary);
  font-size: 0.9rem;
  line-height: 1.5;
}

.login-card__button {
  margin-top: 8px;
  min-height: 46px;
  padding: 12px 20px;
  border-radius: 10px;
  border: 1px solid var(--accent);
  background: var(--accent);
  color: var(--text-inverse);
  font-size: 0.92rem;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 150ms ease;
}

.login-card__button:hover {
  opacity: 0.9;
}
</style>
