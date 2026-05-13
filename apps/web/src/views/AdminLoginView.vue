<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { sessionToken, rememberToken } from '../lib/admin';

const token = ref('');
const remember = ref(false);
const error = ref('');
const router = useRouter();

function submit() {
  error.value = '';
  const value = token.value.trim();
  if (!value) {
    error.value = 'Token is required.';
    return;
  }
  if (remember.value) {
    rememberToken(value);
  } else {
    sessionToken(value);
  }
  void router.push({ name: 'admin-labs' });
}
</script>

<template>
  <div class="admin-login" data-testid="admin-login">
    <div class="admin-login__card">
      <div class="admin-login__icon">🔑</div>
      <h1 class="admin-login__title">Admin Access</h1>
      <p class="admin-login__desc">Enter your admin token to continue</p>

      <form class="admin-login__form" @submit.prevent="submit">
        <label class="field field--stacked">
          <span>Token</span>
          <input
            v-model="token"
            type="password"
            autocomplete="current-password"
            placeholder="admin_sk_…"
          />
        </label>

        <label class="admin-login__remember">
          <input v-model="remember" type="checkbox" />
          <span>Remember on this device</span>
          <span class="admin-login__remember-note">(uses localStorage — only on trusted devices)</span>
        </label>

        <p v-if="error" class="admin-login__error">{{ error }}</p>

        <button type="submit" class="button admin-login__submit">Authenticate</button>
      </form>
    </div>
  </div>
</template>

<style scoped>
.admin-login {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: var(--bg-root);
}

.admin-login__card {
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: 14px;
  padding: 32px 28px;
  width: 340px;
  text-align: center;
}

.admin-login__icon {
  font-size: 28px;
  margin-bottom: 10px;
}

.admin-login__title {
  font-size: 1.1rem;
  font-weight: 700;
  margin: 0 0 4px;
}

.admin-login__desc {
  color: var(--text-secondary);
  font-size: 0.82rem;
  margin: 0 0 20px;
}

.admin-login__form {
  display: flex;
  flex-direction: column;
  gap: 14px;
  text-align: left;
}

.admin-login__remember {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 0.82rem;
  cursor: pointer;
}

.admin-login__remember input {
  margin-top: 2px;
  flex-shrink: 0;
}

.admin-login__remember-note {
  color: var(--text-tertiary);
  font-size: 0.75rem;
}

.admin-login__error {
  margin: 0;
  color: var(--danger);
  font-size: 0.82rem;
}

.admin-login__submit {
  width: 100%;
}
</style>
