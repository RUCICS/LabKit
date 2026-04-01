<script setup lang="ts">
import { computed } from 'vue';

const params = new URL(window.location.href).searchParams;
const isWebSessionRecovery = computed(() => params.get('mode') === 'web-session');
const studentId = computed(() => params.get('student_id') ?? '');
const userKeyId = computed(() => params.get('user_key_id') ?? '');
const hasConfirmation = computed(
  () => isWebSessionRecovery.value || (studentId.value !== '' && userKeyId.value !== '')
);
</script>

<template>
  <main class="auth-confirm-view" data-testid="page-shell">
    <section class="auth-confirm-view__panel">
      <div class="auth-confirm-view__brand">
        <span class="auth-confirm-view__icon" aria-hidden="true">L</span>
        <span>LabKit</span>
      </div>
      <p v-if="!hasConfirmation" class="auth-confirm-view__status">
        Missing OAuth confirmation parameters.
      </p>
      <div v-else class="auth-confirm-view__success">
        <h1 class="auth-confirm-view__title">
          {{ isWebSessionRecovery ? 'Browser session restored' : 'Device connected' }}
        </h1>
        <p class="auth-confirm-view__status auth-confirm-view__status--success">
          Confirmation complete
        </p>
        <dl v-if="!isWebSessionRecovery" class="auth-confirm-view__details">
          <div>
            <dt>Student ID</dt>
            <dd>{{ studentId }}</dd>
          </div>
          <div>
            <dt>User key</dt>
            <dd>Key {{ userKeyId }}</dd>
          </div>
        </dl>
      </div>
    </section>
  </main>
</template>

<style scoped>
.auth-confirm-view {
  min-height: calc(100vh - 180px);
  display: grid;
  place-items: center;
}

.auth-confirm-view__panel {
  width: min(560px, 100%);
  padding: 32px 24px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
  box-shadow: var(--shadow);
}

.auth-confirm-view__brand {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
  font-family: var(--font-mono);
  font-size: 0.95rem;
  font-weight: 800;
}

.auth-confirm-view__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: var(--accent);
  color: var(--text-inverse);
}

.auth-confirm-view__title {
  margin: 0 0 12px;
  font-family: var(--font-mono);
  font-size: 1.7rem;
  font-weight: 700;
  letter-spacing: -0.04em;
}

.auth-confirm-view__status {
  margin: 0;
  color: var(--text-secondary);
}

.auth-confirm-view__status--success {
  color: var(--accent);
  text-transform: uppercase;
  letter-spacing: 0.1em;
  font-size: 0.72rem;
  font-family: var(--font-mono);
  font-weight: 600;
}

.auth-confirm-view__details {
  display: grid;
  gap: 16px;
  margin: 16px 0 0;
}

.auth-confirm-view__details dt {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.auth-confirm-view__details dd {
  margin: 4px 0 0;
  font-family: var(--font-mono);
  font-size: 1rem;
  font-weight: 700;
}
</style>
