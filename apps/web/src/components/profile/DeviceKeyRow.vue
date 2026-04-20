<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue';

type DeviceKey = {
  id: number;
  public_key: string;
  device_name: string;
  created_at: string;
};

const props = defineProps<{
  deviceKey: DeviceKey;
  createdAt: string;
}>();

const copyStatus = ref<'idle' | 'copied' | 'failed'>('idle');
let copyResetTimer: ReturnType<typeof window.setTimeout> | null = null;
let isMounted = true;

const copyLabel = computed(() => {
  if (copyStatus.value === 'copied') return 'Copied';
  if (copyStatus.value === 'failed') return 'Copy failed';
  return 'Copy key';
});

async function writeClipboard(text: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }

  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.setAttribute('readonly', 'true');
  textarea.style.position = 'fixed';
  textarea.style.top = '-9999px';
  textarea.style.left = '-9999px';
  document.body.appendChild(textarea);
  textarea.select();

  const ok = document.execCommand?.('copy');
  textarea.remove();
  if (!ok) throw new Error('copy failed');
}

async function copyKey() {
  if (copyResetTimer) {
    window.clearTimeout(copyResetTimer);
    copyResetTimer = null;
  }
  copyStatus.value = 'idle';
  try {
    await writeClipboard(props.deviceKey.public_key);
    if (!isMounted) return;
    copyStatus.value = 'copied';
  } catch {
    if (!isMounted) return;
    copyStatus.value = 'failed';
  } finally {
    if (!isMounted) return;
    copyResetTimer = window.setTimeout(() => {
      copyStatus.value = 'idle';
      copyResetTimer = null;
    }, 1400);
  }
}

onBeforeUnmount(() => {
  isMounted = false;
  if (copyResetTimer) {
    window.clearTimeout(copyResetTimer);
    copyResetTimer = null;
  }
});
</script>

<template>
  <div class="device-key-row">
    <div class="device-key-row__copy">
      <p class="device-key-row__title">{{ deviceKey.device_name }}</p>
      <p class="device-key-row__subtitle device-key-row__mono">
        {{ deviceKey.public_key }}
      </p>
    </div>

    <div class="device-key-row__meta">
      <span class="device-key-row__tag">Key {{ deviceKey.id }}</span>
      <span class="device-key-row__tag">Created {{ createdAt }}</span>
      <button class="device-key-row__copy-button" type="button" @click="copyKey">
        {{ copyLabel }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.device-key-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 14px;
  min-height: 44px;
  padding: 12px 14px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-elevated);
}

.device-key-row__copy {
  min-width: 0;
  display: grid;
  gap: 4px;
  flex: 1 1 14rem;
}

.device-key-row__title,
.device-key-row__subtitle {
  margin: 0;
}

.device-key-row__title {
  min-width: 0;
  font-family: var(--font-mono);
  font-size: 0.8rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.device-key-row__subtitle {
  min-width: 0;
  color: var(--text-secondary);
  font-size: 0.9rem;
  line-height: 1.55;
}

.device-key-row__mono {
  overflow-wrap: anywhere;
  font-family: var(--font-mono);
  color: var(--accent);
}

.device-key-row__meta {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.device-key-row__tag {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.device-key-row__copy-button {
  min-height: 32px;
  padding: 6px 10px;
  border-radius: 999px;
  border: 1px solid var(--border-default);
  background: transparent;
  color: var(--text-tertiary);
  cursor: pointer;
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.device-key-row__copy-button:hover {
  color: var(--text-primary);
  border-color: color-mix(in srgb, var(--accent) 30%, var(--border-default));
}

.device-key-row__copy-button:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

@media (max-width: 480px) {
  .device-key-row__meta {
    width: 100%;
    margin-left: 0;
    justify-content: flex-start;
  }
}
</style>

