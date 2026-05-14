<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue';

type Action = 'reset-daily' | 'grant-bonus' | 'reset-bonus';

defineProps<{
  labId: string;
}>();

const emit = defineEmits<{
  (e: 'pick', action: Action): void;
}>();

const open = ref(false);
const root = ref<HTMLElement | null>(null);

function toggle() {
  open.value = !open.value;
}

function close() {
  open.value = false;
}

function pick(action: Action) {
  close();
  emit('pick', action);
}

function onDocClick(event: MouseEvent) {
  if (!open.value || !root.value) return;
  if (event.target instanceof Node && root.value.contains(event.target)) return;
  close();
}

function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') close();
}

watch(open, (next) => {
  if (next) {
    document.addEventListener('click', onDocClick);
    document.addEventListener('keydown', onKey);
  } else {
    document.removeEventListener('click', onDocClick);
    document.removeEventListener('keydown', onKey);
  }
});

onBeforeUnmount(() => {
  document.removeEventListener('click', onDocClick);
  document.removeEventListener('keydown', onKey);
});
</script>

<template>
  <div ref="root" class="lab-actions" :data-testid="`lab-actions-${labId}`">
    <button
      type="button"
      class="button button--secondary lab-actions__trigger"
      :aria-expanded="open"
      :aria-haspopup="true"
      :aria-label="`More actions for ${labId}`"
      :data-testid="`lab-actions-trigger-${labId}`"
      @click.stop="toggle"
    >
      ⋯
    </button>
    <div v-if="open" class="lab-actions__menu" role="menu">
      <button
        type="button"
        class="lab-actions__item"
        role="menuitem"
        :data-testid="`lab-actions-${labId}-reset-daily`"
        @click="pick('reset-daily')"
      >
        <span class="lab-actions__icon" aria-hidden="true">⟲</span>
        <span class="lab-actions__label">
          Reset daily quota
          <span class="lab-actions__sub">Clears today's usage</span>
        </span>
      </button>
      <button
        type="button"
        class="lab-actions__item"
        role="menuitem"
        :data-testid="`lab-actions-${labId}-grant-bonus`"
        @click="pick('grant-bonus')"
      >
        <span class="lab-actions__icon" aria-hidden="true">✦</span>
        <span class="lab-actions__label">
          Grant bonus quota
          <span class="lab-actions__sub">Adds credits to every participant</span>
        </span>
      </button>
      <div class="lab-actions__divider" role="separator" />
      <button
        type="button"
        class="lab-actions__item lab-actions__item--danger"
        role="menuitem"
        :data-testid="`lab-actions-${labId}-reset-bonus`"
        @click="pick('reset-bonus')"
      >
        <span class="lab-actions__icon" aria-hidden="true">⚠</span>
        <span class="lab-actions__label">
          Reset bonus to 0
          <span class="lab-actions__sub">Wipes all bonus credits for this lab</span>
        </span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.lab-actions {
  position: relative;
}

.lab-actions__trigger {
  min-width: 36px;
  padding: 0;
  font-size: 1rem;
  letter-spacing: 0;
}

.lab-actions__menu {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  z-index: 30;
  min-width: 260px;
  padding: 4px;
  display: flex;
  flex-direction: column;
  gap: 0;
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: 10px;
  box-shadow: 0 14px 32px rgba(0, 0, 0, 0.28);
}

.lab-actions__item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  background: transparent;
  border: 0;
  border-radius: 6px;
  color: var(--text-primary);
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.lab-actions__item:hover {
  background: var(--bg-hover);
}

.lab-actions__item--danger {
  color: var(--danger);
}

.lab-actions__icon {
  width: 16px;
  flex-shrink: 0;
  text-align: center;
  margin-top: 2px;
  color: var(--text-tertiary);
}

.lab-actions__item--danger .lab-actions__icon {
  color: var(--danger);
}

.lab-actions__label {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 0.85rem;
  font-weight: 600;
}

.lab-actions__sub {
  color: var(--text-tertiary);
  font-size: 0.72rem;
  font-weight: 400;
}

.lab-actions__divider {
  height: 1px;
  margin: 4px 6px;
  background: var(--border-default);
}
</style>
