<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { LogIn, ShieldCheck } from 'lucide-vue-next';

const route = useRoute();
const checking = ref(true);
const loggedIn = ref(false);
const studentId = ref('');

const nextPath = computed(() => {
  const raw = route.query.next;
  const value = typeof raw === 'string' ? raw : '';
  if (value && value.startsWith('/') && !value.startsWith('//')) {
    return value;
  }
  return '/grade';
});

function startLogin() {
  // Full-page navigation: web login goes through the school OAuth redirect.
  window.location.href = `/auth/login?next=${encodeURIComponent(nextPath.value)}`;
}

function continueToNext() {
  window.location.href = nextPath.value;
}

async function checkSession() {
  checking.value = true;
  try {
    const response = await fetch('/api/profile', { credentials: 'include' });
    if (response.ok) {
      const payload = (await response.json()) as { student_id?: string };
      loggedIn.value = true;
      studentId.value = payload.student_id ?? '';
    } else {
      loggedIn.value = false;
    }
  } catch {
    loggedIn.value = false;
  } finally {
    checking.value = false;
  }
}

onMounted(() => {
  void checkSession();
});
</script>

<template>
  <main class="login-view" data-testid="login-view">
    <div class="login-card">
      <div class="login-logo">
        <div class="login-logo__icon"><ShieldCheck :size="20" :stroke-width="2.4" /></div>
        <span class="login-logo__text">LabKit</span>
      </div>

      <div class="login-panel">
        <h1 class="login-title">登录查看成绩</h1>
        <p class="login-lede">使用微人大(统一身份认证)登录，无需 CLI 或密钥。</p>

        <p v-if="checking" class="login-status">正在检查登录状态…</p>

        <template v-else-if="loggedIn">
          <p class="login-status">
            已登录{{ studentId ? `为 ${studentId}` : '' }}。
          </p>
          <button class="login-button" type="button" @click="continueToNext">
            <LogIn :size="16" :stroke-width="2.4" />
            <span>继续查看成绩</span>
          </button>
        </template>

        <template v-else>
          <button class="login-button" type="button" data-testid="login-cas" @click="startLogin">
            <LogIn :size="16" :stroke-width="2.4" />
            <span>微人大登录</span>
          </button>
          <p class="login-hint">登录后将跳转回 <code>{{ nextPath }}</code>。</p>
        </template>
      </div>
    </div>
  </main>
</template>

<style scoped>
.login-view {
  min-height: 60vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
}

.login-card {
  width: 100%;
  max-width: 420px;
}

.login-logo {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  margin-bottom: 28px;
}

.login-logo__icon {
  width: 36px;
  height: 36px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--accent);
  color: var(--text-inverse);
}

.login-logo__text {
  font-family: var(--font-mono);
  font-size: 1.3rem;
  font-weight: 700;
  letter-spacing: -0.03em;
}

.login-panel {
  display: grid;
  gap: 14px;
  padding: 28px 26px;
  border: 1px solid var(--border-default);
  border-radius: 12px;
  background: var(--bg-surface);
  text-align: center;
}

.login-title {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 1.1rem;
  font-weight: 700;
}

.login-lede {
  margin: 0;
  color: var(--text-secondary);
  font-size: 0.9rem;
  line-height: 1.5;
}

.login-status {
  margin: 0;
  color: var(--text-secondary);
  font-size: 0.9rem;
}

.login-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 46px;
  padding: 12px 18px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--accent) 40%, var(--border-default));
  background: color-mix(in srgb, var(--accent) 18%, var(--bg-surface));
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 0.82rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  cursor: pointer;
}

.login-button:hover {
  border-color: var(--accent);
}

.login-hint {
  margin: 0;
  color: var(--text-tertiary);
  font-size: 0.78rem;
}

.login-hint code {
  font-family: var(--font-mono);
}
</style>
