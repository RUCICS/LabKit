<script setup lang="ts">
import { ref } from 'vue';
import { RouterLink, useRouter } from 'vue-router';
import { readAdminToken, sessionToken, clearAdminToken } from '../../lib/admin';

const router = useRouter();
const tokenInputVisible = ref(false);
const tokenDraft = ref('');

const isAuthenticated = () => readAdminToken() !== '';

function showTokenInput() {
  tokenDraft.value = '';
  tokenInputVisible.value = true;
}

function saveToken() {
  const value = tokenDraft.value.trim();
  if (value) {
    sessionToken(value);
  }
  tokenInputVisible.value = false;
}

function logout() {
  clearAdminToken();
  void router.push({ name: 'admin-login' });
}
</script>

<template>
  <div class="admin-shell" data-testid="admin-shell">
    <nav class="admin-shell__sidebar">
      <div class="admin-shell__brand">ADMIN</div>

      <RouterLink
        :to="{ name: 'admin-labs' }"
        class="admin-shell__nav-item"
        active-class="admin-shell__nav-item--active"
      >
        🧪 Labs
      </RouterLink>

      <div class="admin-shell__token-slot">
        <div class="admin-shell__token-label">TOKEN</div>
        <template v-if="!tokenInputVisible">
          <div class="admin-shell__token-status">
            <span v-if="isAuthenticated()" class="admin-shell__token-ok">● active</span>
            <span v-else class="admin-shell__token-missing">● none</span>
            <div class="admin-shell__token-actions">
              <button type="button" class="admin-shell__icon-btn" title="Change token" @click="showTokenInput">✎</button>
              <button type="button" class="admin-shell__icon-btn" title="Logout" @click="logout">⏻</button>
            </div>
          </div>
        </template>
        <template v-else>
          <form class="admin-shell__token-form" @submit.prevent="saveToken">
            <input
              v-model="tokenDraft"
              type="password"
              class="admin-shell__token-input"
              placeholder="New token…"
              autofocus
            />
            <button type="submit" class="admin-shell__token-save">✓</button>
          </form>
        </template>
      </div>
    </nav>

    <main class="admin-shell__content">
      <slot />
    </main>
  </div>
</template>

<style scoped>
.admin-shell {
  display: flex;
  min-height: 100vh;
}

.admin-shell__sidebar {
  width: 120px;
  background: var(--bg-surface);
  border-right: 1px solid var(--border-default);
  padding: 16px 10px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex-shrink: 0;
}

.admin-shell__brand {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.12em;
  margin-bottom: 12px;
  padding: 0 6px;
}

.admin-shell__nav-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  border-radius: 6px;
  font-size: 0.85rem;
  color: var(--text-secondary);
  text-decoration: none;
  transition: background 100ms;
}

.admin-shell__nav-item:hover {
  background: var(--bg-elevated);
}

.admin-shell__nav-item--active {
  background: color-mix(in srgb, var(--accent) 12%, transparent);
  color: var(--accent-strong);
}

.admin-shell__token-slot {
  margin-top: auto;
  border-top: 1px solid var(--border-default);
  padding-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.admin-shell__token-label {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.65rem;
  letter-spacing: 0.1em;
}

.admin-shell__token-status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--bg-elevated);
  border-radius: 4px;
  padding: 4px 6px;
}

.admin-shell__token-ok {
  color: var(--accent-strong);
  font-family: var(--font-mono);
  font-size: 0.68rem;
}

.admin-shell__token-missing {
  color: var(--danger);
  font-family: var(--font-mono);
  font-size: 0.68rem;
}

.admin-shell__token-actions {
  display: flex;
  gap: 2px;
}

.admin-shell__icon-btn {
  background: none;
  border: none;
  color: var(--text-tertiary);
  cursor: pointer;
  padding: 0 2px;
  font-size: 0.82rem;
  line-height: 1;
}

.admin-shell__icon-btn:hover {
  color: var(--text-secondary);
}

.admin-shell__token-form {
  display: flex;
  gap: 4px;
}

.admin-shell__token-input {
  flex: 1;
  min-width: 0;
  padding: 3px 5px;
  font-size: 0.75rem;
  background: var(--bg-root);
  border: 1px solid var(--border-default);
  border-radius: 3px;
  color: var(--text-primary);
}

.admin-shell__token-save {
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: 3px;
  padding: 3px 6px;
  cursor: pointer;
  font-size: 0.8rem;
}

.admin-shell__content {
  flex: 1;
  min-width: 0;
  padding: 24px;
  overflow-y: auto;
}
</style>
