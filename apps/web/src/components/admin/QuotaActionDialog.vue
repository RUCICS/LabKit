<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { authorizedAdminHeaders, readAPIError } from '../../lib/admin';

type Action = 'reset-daily' | 'grant-bonus' | 'reset-bonus';

const props = defineProps<{
  labId: string;
  labName: string;
  action: Action | null;
}>();

const emit = defineEmits<{
  (e: 'close'): void;
  (e: 'done', summary: string): void;
}>();

const delta = ref<string>('5');
const previewLoading = ref(false);
const previewError = ref<string | null>(null);
const participants = ref<number | null>(null);
const submitting = ref(false);
const submitError = ref<string | null>(null);

const isOpen = computed(() => props.action !== null);

const config = computed(() => {
  switch (props.action) {
    case 'reset-daily':
      return {
        title: 'Reset daily quota',
        destructive: true,
        confirmLabel: 'Reset',
        endpoint: () => `/api/admin/labs/${encodeURIComponent(props.labId)}/quota/reset`,
        previewEndpoint: null as null | (() => string),
        previewPayload: null as null | object,
        applyPayload: () => ({} as Record<string, unknown>),
        description:
          "Clears today's quota usage for everyone in this lab. Existing submissions stay in the audit log. Cannot be undone.",
        needsDelta: false,
      };
    case 'grant-bonus':
      return {
        title: 'Grant bonus quota',
        destructive: false,
        confirmLabel: 'Apply',
        endpoint: () => `/api/admin/labs/${encodeURIComponent(props.labId)}/quota/bonus`,
        previewEndpoint: () => `/api/admin/labs/${encodeURIComponent(props.labId)}/quota/bonus`,
        previewPayload: { dry_run: true, delta: parseDelta() } as Record<string, unknown>,
        applyPayload: () => ({ delta: parseDelta() } as Record<string, unknown>),
        description:
          'Adds the chosen amount to every participant’s bonus.remaining. Negative values reduce remaining bonus (floors at 0). 0 is not allowed.',
        needsDelta: true,
      };
    case 'reset-bonus':
      return {
        title: 'Reset bonus quota',
        destructive: true,
        confirmLabel: 'Reset',
        endpoint: () => `/api/admin/labs/${encodeURIComponent(props.labId)}/quota/bonus/reset`,
        previewEndpoint: () => `/api/admin/labs/${encodeURIComponent(props.labId)}/quota/bonus/reset`,
        previewPayload: { dry_run: true } as Record<string, unknown>,
        applyPayload: () => ({} as Record<string, unknown>),
        description:
          'Sets bonus.remaining to 0 for every participant of this lab. Cannot be undone.',
        needsDelta: false,
      };
    default:
      return null;
  }
});

function parseDelta(): number {
  const n = Number.parseInt(delta.value, 10);
  return Number.isFinite(n) ? n : 0;
}

const deltaValid = computed(() => {
  if (!config.value?.needsDelta) return true;
  const n = parseDelta();
  return Number.isInteger(n) && n !== 0 && Math.abs(n) <= 999;
});

const confirmLabel = computed(() => {
  if (!config.value) return '';
  if (config.value.needsDelta) {
    const n = parseDelta();
    if (n > 0) return `Grant +${n}`;
    if (n < 0) return `Revoke ${Math.abs(n)}`;
    return config.value.confirmLabel;
  }
  return config.value.confirmLabel;
});

watch(
  () => props.action,
  async (next) => {
    if (next === null) return;
    delta.value = '5';
    participants.value = null;
    previewError.value = null;
    submitError.value = null;
    submitting.value = false;
    await runPreview();
  },
  { immediate: true },
);

watch(delta, async (_, prev) => {
  if (!isOpen.value || !config.value?.needsDelta || prev === delta.value) return;
  if (!deltaValid.value) {
    return;
  }
  await runPreview();
});

async function runPreview() {
  const cfg = config.value;
  if (!cfg?.previewEndpoint || !cfg.previewPayload) {
    // Reset-daily has no preview endpoint; fall back to fetching participant
    // count via the bonus dry-run (which shares CountLabParticipants).
    await previewParticipants();
    return;
  }
  previewLoading.value = true;
  previewError.value = null;
  try {
    const payload = props.action === 'grant-bonus'
      ? { ...cfg.previewPayload, delta: parseDelta() }
      : cfg.previewPayload;
    const res = await fetch(cfg.previewEndpoint(), {
      method: 'POST',
      headers: authorizedAdminHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify(payload),
    });
    if (!res.ok) {
      previewError.value = await readAPIError(res, 'Preview failed');
      return;
    }
    const data = (await res.json()) as { lab_participants?: number };
    participants.value = data.lab_participants ?? null;
  } catch {
    previewError.value = 'Preview failed';
  } finally {
    previewLoading.value = false;
  }
}

async function previewParticipants() {
  previewLoading.value = true;
  previewError.value = null;
  try {
    // Cheapest available count source: bonus dry-run with a no-op delta.
    // Server validates delta != 0, so we send 1 and ignore the response side.
    const res = await fetch(`/api/admin/labs/${encodeURIComponent(props.labId)}/quota/bonus`, {
      method: 'POST',
      headers: authorizedAdminHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify({ delta: 1, dry_run: true }),
    });
    if (!res.ok) {
      previewError.value = await readAPIError(res, 'Preview failed');
      return;
    }
    const data = (await res.json()) as { lab_participants?: number };
    participants.value = data.lab_participants ?? null;
  } catch {
    previewError.value = 'Preview failed';
  } finally {
    previewLoading.value = false;
  }
}

async function apply() {
  const cfg = config.value;
  if (!cfg) return;
  if (cfg.needsDelta && !deltaValid.value) return;
  submitting.value = true;
  submitError.value = null;
  try {
    const init: RequestInit = {
      method: 'POST',
      headers: authorizedAdminHeaders({ 'Content-Type': 'application/json' }),
    };
    if (cfg.needsDelta) {
      init.body = JSON.stringify(cfg.applyPayload());
    } else if (props.action === 'reset-bonus') {
      init.body = JSON.stringify({});
    }
    const res = await fetch(cfg.endpoint(), init);
    if (!res.ok) {
      submitError.value = await readAPIError(res, 'Action failed');
      return;
    }
    const data = (await res.json()) as Record<string, unknown>;
    emit('done', summarize(cfg.title, data));
  } catch {
    submitError.value = 'Network error';
  } finally {
    submitting.value = false;
  }
}

function summarize(title: string, data: Record<string, unknown>): string {
  const affected = (data.users_affected ?? data.rows_affected) as number | undefined;
  if (typeof affected === 'number') {
    return `${title}: ${affected} row(s) updated.`;
  }
  return `${title}: done.`;
}

function onClose() {
  if (submitting.value) return;
  emit('close');
}
</script>

<template>
  <Transition name="dialog">
    <div
      v-if="isOpen && config"
      class="dialog-overlay"
      data-testid="quota-action-dialog"
      @click.self="onClose"
    >
      <div class="dialog" role="dialog" aria-modal="true">
        <header class="dialog__head">
          <h2 class="dialog__title">{{ config.title }}</h2>
          <button type="button" class="dialog__close" :disabled="submitting" @click="onClose">✕</button>
        </header>

        <dl class="dialog__meta">
          <dt>Lab</dt>
          <dd>
            <span>{{ labName }}</span>
            <code>{{ labId }}</code>
          </dd>
        </dl>

        <div v-if="config.needsDelta" class="dialog__field">
          <label class="dialog__field-label" for="delta-input">Each participant gets</label>
          <input
            id="delta-input"
            v-model="delta"
            class="dialog__input"
            type="number"
            inputmode="numeric"
            step="1"
            data-testid="delta-input"
          />
          <p class="dialog__hint">
            Negative values revoke bonus credits (floors at 0). Zero is not allowed.
          </p>
        </div>

        <p :class="['dialog__desc', config.destructive ? 'dialog__desc--warn' : '']">
          <span v-if="config.destructive" aria-hidden="true">⚠ </span>{{ config.description }}
        </p>

        <div class="dialog__preview" data-testid="dialog-preview">
          <template v-if="previewLoading">
            <span class="dialog__preview-label">Counting participants…</span>
          </template>
          <template v-else-if="previewError">
            <span class="dialog__preview-error">{{ previewError }}</span>
          </template>
          <template v-else-if="participants !== null">
            <span class="dialog__preview-count" data-testid="participant-count">{{ participants }}</span>
            <span class="dialog__preview-label">
              participant{{ participants === 1 ? '' : 's' }} affected
              <span class="dialog__preview-sub">(users with ≥ 1 submission to this lab)</span>
            </span>
          </template>
        </div>

        <p v-if="submitError" class="dialog__error">{{ submitError }}</p>

        <footer class="dialog__foot">
          <button type="button" class="button button--secondary" :disabled="submitting" @click="onClose">
            Cancel
          </button>
          <button
            type="button"
            :class="['button', config.destructive ? 'button--danger' : '']"
            :disabled="submitting || (config.needsDelta && !deltaValid)"
            data-testid="dialog-confirm"
            @click="apply"
          >
            {{ submitting ? 'Working…' : confirmLabel }}
          </button>
        </footer>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.dialog-overlay {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.55);
  padding: 24px;
}

.dialog {
  width: 100%;
  max-width: 440px;
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: 12px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.35);
}

.dialog__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dialog__title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 700;
}

.dialog__close {
  background: transparent;
  border: 0;
  color: var(--text-tertiary);
  cursor: pointer;
  font-size: 1rem;
  padding: 4px 6px;
}

.dialog__close:hover:not(:disabled) {
  color: var(--text-primary);
}

.dialog__meta {
  display: grid;
  grid-template-columns: 60px 1fr;
  gap: 4px 12px;
  margin: 0;
  font-size: 0.78rem;
}

.dialog__meta dt {
  color: var(--text-tertiary);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  font-size: 0.7rem;
}

.dialog__meta dd {
  margin: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: baseline;
}

.dialog__meta code {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
}

.dialog__field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.dialog__field-label {
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-secondary);
}

.dialog__input {
  height: 36px;
  padding: 0 10px;
  border: 1px solid var(--border-default);
  border-radius: 6px;
  background: var(--bg-elevated);
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 0.92rem;
  width: 120px;
}

.dialog__input:focus {
  outline: none;
  border-color: var(--border-strong);
  box-shadow: var(--focus-ring);
}

.dialog__hint {
  margin: 0;
  font-size: 0.72rem;
  color: var(--text-tertiary);
}

.dialog__desc {
  margin: 0;
  font-size: 0.82rem;
  color: var(--text-secondary);
  line-height: 1.5;
}

.dialog__desc--warn {
  color: var(--danger);
}

.dialog__preview {
  padding: 12px;
  border: 1px dashed var(--border-default);
  border-radius: 8px;
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 8px;
  min-height: 40px;
}

.dialog__preview-count {
  font-size: 1.3rem;
  font-weight: 700;
  color: var(--text-primary);
  font-family: var(--font-mono);
}

.dialog__preview-label {
  color: var(--text-secondary);
  font-size: 0.82rem;
}

.dialog__preview-sub {
  display: block;
  color: var(--text-tertiary);
  font-size: 0.72rem;
}

.dialog__preview-error {
  color: var(--danger);
  font-size: 0.82rem;
}

.dialog__error {
  margin: 0;
  color: var(--danger);
  font-size: 0.82rem;
}

.dialog__foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.button--danger {
  background: var(--danger);
  border-color: var(--danger);
  color: var(--text-primary);
}

.button--danger:hover:not(:disabled) {
  background: var(--danger);
  border-color: var(--danger);
  filter: brightness(1.1);
}

.dialog-enter-active,
.dialog-leave-active {
  transition: opacity 120ms ease;
}

.dialog-enter-from,
.dialog-leave-to {
  opacity: 0;
}
</style>
