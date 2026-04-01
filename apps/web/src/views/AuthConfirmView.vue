<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { Check, ShieldCheck, TriangleAlert } from 'lucide-vue-next';

const params = new URLSearchParams(window.location.search);
const isWebSessionRecovery = computed(() => params.get('mode') === 'web-session');
const studentId = computed(() => params.get('student_id') ?? '');
const userKeyId = computed(() => params.get('user_key_id') ?? '');
const hasConfirmation = computed(
  () => isWebSessionRecovery.value || (studentId.value !== '' && userKeyId.value !== '')
);

const secondsRemaining = ref(5);
const autoCloseFailed = ref(false);
let countdownTimer: number | null = null;
let closeFallbackTimer: number | null = null;

function clearTimers() {
  if (countdownTimer !== null) {
    window.clearInterval(countdownTimer);
    countdownTimer = null;
  }
  if (closeFallbackTimer !== null) {
    window.clearTimeout(closeFallbackTimer);
    closeFallbackTimer = null;
  }
}

onMounted(() => {
  if (!hasConfirmation.value) {
    return;
  }

  countdownTimer = window.setInterval(() => {
    secondsRemaining.value -= 1;
    if (secondsRemaining.value > 0) {
      return;
    }
    clearTimers();
    window.close();
    closeFallbackTimer = window.setTimeout(() => {
      autoCloseFailed.value = true;
    }, 200);
  }, 1000);
});

onBeforeUnmount(() => {
  clearTimers();
});
</script>

<template>
  <main class="auth-confirm-page" :class="{ success: hasConfirmation }">
    <div class="auth-card">
      <div class="auth-logo">
        <div class="logo-icon"><ShieldCheck :size="20" :stroke-width="2.4" /></div>
        <span class="logo-text">LabKit</span>
      </div>

      <div class="auth-form">
        <div v-if="hasConfirmation" class="success-overlay visible">
          <div class="success-icon">
            <Check :size="28" :stroke-width="2.6" />
          </div>
          <span class="success-text">
            {{ isWebSessionRecovery ? 'Browser session restored' : '设备已授权' }}
          </span>
          <p class="success-detail">
            <template v-if="isWebSessionRecovery">
              你现在可以回到 LabKit 继续操作
            </template>
            <template v-else>
              公钥已绑定至学号 <strong>{{ studentId }}</strong><br>
              你现在可以回到终端使用 LabKit
            </template>
          </p>
          <div v-if="!isWebSessionRecovery" class="success-meta">Key {{ userKeyId }}</div>
          <span class="success-countdown">
            {{
              autoCloseFailed
                ? '如果页面未自动关闭，可直接关闭此标签页'
                : `页面将在 ${Math.max(secondsRemaining, 0)} 秒后关闭`
            }}
          </span>
        </div>

        <div v-else class="error-view">
          <div class="error-icon"><TriangleAlert :size="26" :stroke-width="2.4" /></div>
          <h1 class="error-title">授权未完成</h1>
          <p class="error-detail">缺少必要参数，请重新回到终端发起授权。</p>
        </div>
      </div>
    </div>
  </main>
</template>

<style scoped>
.auth-confirm-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.auth-confirm-page::before {
  content: '';
  position: fixed;
  inset: 0;
  background:
    linear-gradient(rgba(148, 163, 194, 0.025) 1px, transparent 1px),
    linear-gradient(90deg, rgba(148, 163, 194, 0.025) 1px, transparent 1px);
  background-size: 48px 48px;
  animation: gridDrift 20s linear infinite;
  pointer-events: none;
}

.auth-confirm-page::after {
  content: '';
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -60%);
  width: 600px;
  height: 400px;
  background: radial-gradient(ellipse, rgba(245, 158, 11, 0.25) 0%, transparent 70%);
  opacity: 0.18;
  pointer-events: none;
}

.auth-confirm-page.success::after {
  background: radial-gradient(ellipse, rgba(52, 211, 153, 0.25) 0%, transparent 70%);
  opacity: 0.28;
}

.auth-card {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 440px;
  padding: 0 24px;
  animation: cardEnter 0.5s ease both;
}

.auth-logo {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  margin-bottom: 48px;
}

.logo-icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f59e0b;
  color: #06090f;
}

.logo-text {
  font-family: var(--font-mono);
  font-size: 1.4rem;
  font-weight: 700;
  letter-spacing: -0.03em;
}

.auth-form {
  background: #0b1120;
  border: 1px solid rgba(148, 163, 194, 0.1);
  border-radius: 12px;
  padding: 36px 32px;
  text-align: center;
}

.success-overlay {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  padding: 20px 0 8px;
  animation: fadeIn 0.4s ease both;
}

.success-icon {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  background: rgba(52, 211, 153, 0.12);
  border: 2px solid rgba(52, 211, 153, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  animation: successPop 0.5s ease both;
}

.success-text {
  font-family: var(--font-mono);
  font-size: 1rem;
  font-weight: 600;
  color: #34d399;
}

.success-detail {
  font-size: 0.85rem;
  color: #8494a7;
  line-height: 1.5;
  text-align: center;
}

.success-detail strong,
.success-meta {
  color: #e2e8f0;
  font-weight: 600;
}

.success-meta {
  font-family: var(--font-mono);
  font-size: 0.78rem;
}

.success-countdown {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  color: #4a5568;
  margin-top: 4px;
}

.error-view {
  display: grid;
  gap: 14px;
  justify-items: center;
}

.error-icon {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #f87171;
  background: rgba(248, 113, 113, 0.12);
  border: 2px solid rgba(248, 113, 113, 0.22);
}

.error-title {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 1rem;
}

.error-detail {
  margin: 0;
  color: #8494a7;
  font-size: 0.85rem;
}

@keyframes gridDrift {
  0% { background-position: 0 0, 0 0; }
  100% { background-position: 48px 48px, 48px 48px; }
}

@keyframes cardEnter {
  from { opacity: 0; transform: translateY(12px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

@keyframes successPop {
  0% { transform: scale(0.5); opacity: 0; }
  60% { transform: scale(1.1); }
  100% { transform: scale(1); opacity: 1; }
}

@media (max-width: 480px) {
  .auth-card {
    padding: 0 16px;
  }

  .auth-form {
    padding: 28px 20px;
  }

  .auth-logo {
    margin-bottom: 36px;
  }
}
</style>
