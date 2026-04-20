<script setup lang="ts">
import { computed, nextTick, ref } from 'vue';
import { ShieldCheck, SquareTerminal } from 'lucide-vue-next';

const rawQueryCode = (new URLSearchParams(window.location.search).get('user_code') ?? '')
  .replace(/[^a-zA-Z0-9]/g, '')
  .toUpperCase()
  .slice(0, 8);

const code = ref(Array.from({ length: 8 }, (_, index) => rawQueryCode[index] ?? ''));
const errorMessage = ref('');
const isSubmitting = ref(false);
const activeIndex = ref(Math.min(rawQueryCode.length, 7));
const inputs = ref<HTMLInputElement[]>([]);

const codeLength = computed(() => code.value.join('').length);
const formattedCode = computed(() => {
  const joined = code.value.join('');
  if (joined.length <= 4) {
    return joined;
  }
  return `${joined.slice(0, 4)}-${joined.slice(4)}`;
});

function setInputRef(index: number) {
  return (el: Element | null) => {
    if (el instanceof HTMLInputElement) {
      inputs.value[index] = el;
    }
  };
}

function focusInput(index: number) {
  const next = inputs.value[index];
  if (!next) {
    return;
  }
  activeIndex.value = index;
  next.focus();
  setTimeout(() => next.select(), 0);
}

function handleInput(index: number, event: Event) {
  const target = event.target as HTMLInputElement | null;
  const nextValue = (target?.value ?? '').replace(/[^a-zA-Z0-9]/g, '').toUpperCase();
  code.value[index] = nextValue.slice(-1);
  if (target) {
    target.value = code.value[index];
  }
  if (errorMessage.value !== '') {
    errorMessage.value = '';
  }
  if (code.value[index] !== '' && index < code.value.length - 1) {
    focusInput(index + 1);
  }
}

function handleKeydown(index: number, event: KeyboardEvent) {
  if (event.key === 'Backspace' && code.value[index] === '' && index > 0) {
    event.preventDefault();
    code.value[index - 1] = '';
    focusInput(index - 1);
    return;
  }
  if (event.key === 'ArrowLeft' && index > 0) {
    event.preventDefault();
    focusInput(index - 1);
    return;
  }
  if (event.key === 'ArrowRight' && index < code.value.length - 1) {
    event.preventDefault();
    focusInput(index + 1);
    return;
  }
  if (event.key === 'Enter' && codeLength.value === 8) {
    event.preventDefault();
    submit();
  }
}

function handlePaste(index: number, event: ClipboardEvent) {
  event.preventDefault();
  const pasted = (event.clipboardData?.getData('text') ?? '')
    .replace(/[^a-zA-Z0-9]/g, '')
    .toUpperCase();
  if (pasted === '') {
    return;
  }
  for (let offset = 0; offset < pasted.length && index + offset < code.value.length; offset += 1) {
    code.value[index + offset] = pasted[offset];
  }
  if (errorMessage.value !== '') {
    errorMessage.value = '';
  }
  const nextIndex = Math.min(index + pasted.length, code.value.length - 1);
  nextTick(() => focusInput(nextIndex));
}

function submit() {
  if (codeLength.value !== 8) {
    errorMessage.value = '验证码无效或已过期，请检查终端中的验证码';
    return;
  }
  isSubmitting.value = true;
  window.location.assign(`/api/device/verify?user_code=${encodeURIComponent(formattedCode.value)}`);
}

function handleSubmit(event: Event) {
  event.preventDefault();
  submit();
}
</script>

<template>
  <main class="device-auth-page">
    <div class="auth-card">
      <div class="auth-logo">
        <div class="logo-icon"><ShieldCheck :size="20" :stroke-width="2.4" /></div>
        <span class="logo-text">LabKit</span>
      </div>

      <div class="auth-form">
        <h1 class="auth-title">设备授权</h1>
        <p class="auth-desc">
          请输入终端中 <code>labkit auth</code> 显示的验证码
        </p>

        <form @submit="handleSubmit">
          <div class="code-group">
            <div class="code-half">
              <input
                v-for="index in 4"
                :key="index - 1"
                :ref="setInputRef(index - 1)"
                class="code-input"
                :class="{
                  filled: code[index - 1] !== '',
                  error: errorMessage !== '',
                  active: activeIndex === index - 1
                }"
                data-testid="user-code-cell"
                type="text"
                maxlength="1"
                autocomplete="off"
                :value="code[index - 1]"
                @focus="focusInput(index - 1)"
                @input="handleInput(index - 1, $event)"
                @keydown="handleKeydown(index - 1, $event)"
                @paste="handlePaste(index - 1, $event)"
              />
            </div>
            <span class="code-sep">–</span>
            <div class="code-half">
              <input
                v-for="index in 4"
                :key="index + 3"
                :ref="setInputRef(index + 3)"
                class="code-input"
                :class="{
                  filled: code[index + 3] !== '',
                  error: errorMessage !== '',
                  active: activeIndex === index + 3
                }"
                data-testid="user-code-cell"
                type="text"
                maxlength="1"
                autocomplete="off"
                :value="code[index + 3]"
                @focus="focusInput(index + 3)"
                @input="handleInput(index + 3, $event)"
                @keydown="handleKeydown(index + 3, $event)"
                @paste="handlePaste(index + 3, $event)"
              />
            </div>
          </div>

          <button class="auth-submit" :class="{ loading: isSubmitting }" :disabled="codeLength < 8 || isSubmitting" type="submit">
            确认授权
          </button>
          <div class="auth-message" :class="{ error: errorMessage }">{{ errorMessage }}</div>
        </form>
      </div>

    </div>

    <div class="terminal-hint">
      <SquareTerminal :size="13" :stroke-width="2.2" aria-hidden="true" />
      <span class="hint-cmd">labkit auth</span>
    </div>
  </main>
</template>

<style scoped>
.device-auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.device-auth-page::before {
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

.device-auth-page::after {
  content: '';
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -60%);
  width: 600px;
  height: 400px;
  background: radial-gradient(ellipse, color-mix(in srgb, var(--accent) 25%, transparent) 0%, transparent 70%);
  opacity: 0.2;
  pointer-events: none;
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
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--accent);
  color: var(--text-inverse);
}

.logo-text {
  font-family: var(--font-mono);
  font-size: 1.4rem;
  font-weight: 700;
  letter-spacing: -0.03em;
}

.auth-form {
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: 10px;
  padding: 36px 32px;
  text-align: center;
}

.auth-title {
  font-family: var(--font-mono);
  font-size: 1rem;
  font-weight: 600;
  margin-bottom: 6px;
  letter-spacing: -0.02em;
}

.auth-desc {
  font-size: 0.85rem;
  color: var(--text-secondary);
  margin-bottom: 32px;
  line-height: 1.5;
}

.auth-desc code {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  background: var(--bg-elevated);
  padding: 2px 6px;
  border-radius: 3px;
  color: var(--text-primary);
  border: 1px solid var(--border-subtle);
}

.code-group {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-bottom: 28px;
  width: 100%;
}

.code-half {
  flex: 1 1 0;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 6px;
  min-width: 0;
}

.code-sep {
  font-family: var(--font-mono);
  font-size: 1.5rem;
  font-weight: 300;
  color: var(--text-tertiary);
  margin: 0 4px;
  user-select: none;
}

.code-input {
  width: 100%;
  height: 56px;
  background: var(--bg-root);
  border: 1.5px solid var(--border-strong);
  border-radius: 6px;
  font-family: var(--font-mono);
  font-size: 1.4rem;
  font-weight: 700;
  color: var(--text-primary);
  text-align: center;
  text-transform: uppercase;
  outline: none;
  transition: border-color 0.15s ease, box-shadow 0.15s ease, background 0.15s ease;
  caret-color: var(--accent);
}

.code-input::placeholder {
  color: var(--text-tertiary);
  opacity: 0.4;
  font-weight: 400;
}

.code-input.active {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 12%, transparent), 0 0 20px color-mix(in srgb, var(--accent) 12%, transparent);
  background: var(--bg-elevated);
}

.code-input.filled {
  border-color: rgba(148, 163, 194, 0.3);
  background: var(--bg-elevated);
}

.code-input.error {
  border-color: var(--color-error);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-error) 12%, transparent);
  animation: inputShake 0.4s ease;
}

.auth-submit {
  width: 100%;
  padding: 12px 24px;
  font-family: var(--font-mono);
  font-size: 0.85rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: var(--text-inverse);
  background: var(--accent);
  border: none;
  border-radius: 6px;
  cursor: pointer;
  transition: filter 0.15s ease, transform 0.15s ease, box-shadow 0.15s ease;
  position: relative;
  overflow: hidden;
}

.auth-submit:hover:not(:disabled) {
  filter: brightness(1.1);
  transform: translateY(-1px);
  box-shadow: 0 4px 16px color-mix(in srgb, var(--accent) 30%, transparent);
}

.auth-submit:active:not(:disabled) {
  transform: translateY(1px) scale(0.98);
}

.auth-submit:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

.auth-submit:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.auth-submit.loading {
  color: transparent;
  pointer-events: none;
}

.auth-submit.loading::after {
  content: '';
  position: absolute;
  top: 50%;
  left: 50%;
  width: 18px;
  height: 18px;
  margin: -9px 0 0 -9px;
  border: 2px solid var(--text-inverse);
  border-top-color: transparent;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

.auth-message {
  margin-top: 16px;
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--text-tertiary);
  min-height: 20px;
  transition: color 0.2s ease;
}

.auth-message.error {
  color: var(--color-error);
}

.terminal-hint {
  position: fixed;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 1;
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: 6px;
  padding: 10px 16px;
  font-family: var(--font-mono);
  font-size: 0.72rem;
  color: var(--text-tertiary);
  display: flex;
  align-items: center;
  gap: 8px;
  animation: hintEnter 0.5s ease 0.3s both;
  white-space: nowrap;
}

.hint-cmd {
  color: var(--text-secondary);
}

@keyframes gridDrift {
  0% { background-position: 0 0, 0 0; }
  100% { background-position: 48px 48px, 48px 48px; }
}

@keyframes cardEnter {
  from { opacity: 0; transform: translateY(12px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes inputShake {
  0%, 100% { transform: translateX(0); }
  20%, 60% { transform: translateX(-4px); }
  40%, 80% { transform: translateX(4px); }
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

@keyframes hintEnter {
  from { opacity: 0; transform: translateX(-50%) translateY(8px); }
  to { opacity: 1; transform: translateX(-50%) translateY(0); }
}

@media (max-width: 480px) {
  .auth-card {
    padding: 0 16px;
  }

  .auth-form {
    padding: 28px 20px;
  }

  .code-input {
    height: 50px;
    font-size: 1.2rem;
  }

  .code-sep {
    font-size: 1.2rem;
    margin: 0 2px;
  }

  .code-half {
    gap: 5px;
  }

  .terminal-hint {
    display: none;
  }

  .auth-logo {
    margin-bottom: 36px;
  }
}
</style>
