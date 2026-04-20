<script setup lang="ts">
withDefaults(
  defineProps<{
    title: string;
    labId: string;
    remainingLabel?: string;
    remainingValue: string;
    closesAtLabel?: string;
    closesAtValue?: string;
  }>(),
  {
    remainingLabel: 'REMAINING',
    closesAtLabel: 'CLOSES'
  }
);
</script>

<template>
  <section class="lab-context">
    <div class="lab-context__left">
      <h1 class="lab-context__title">{{ title }}</h1>
      <p class="lab-context__lab-id">{{ labId }}</p>
      <div v-if="$slots.meta" class="lab-context__meta">
        <slot name="meta" />
      </div>
    </div>

    <div class="lab-context__right">
      <div class="lab-context__remaining">
        <strong class="lab-context__remaining-value">{{ remainingValue }}</strong>
        <span class="lab-context__remaining-label">{{ remainingLabel }}</span>
      </div>

      <div v-if="closesAtValue" class="lab-context__closes">
        <span class="lab-context__closes-label">{{ closesAtLabel }}</span>
        <span class="lab-context__closes-value">{{ closesAtValue }}</span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.lab-context {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  padding: 8px 2px;
}

.lab-context__left {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.lab-context__title {
  margin: 0;
  font-family: var(--font-mono);
  font-size: clamp(1.4rem, 4.2vw, 1.7rem);
  font-weight: 700;
  letter-spacing: -0.04em;
  line-height: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lab-context__lab-id {
  margin: 0;
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.lab-context__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.04em;
}

.lab-context__meta > :deep(*) {
  display: inline-flex;
  gap: 6px;
  white-space: nowrap;
}

.lab-context__meta > :deep(*:not(:last-child))::after {
  content: '·';
  color: var(--border-strong);
  margin-left: 6px;
}

.lab-context__right {
  display: grid;
  justify-items: end;
  gap: 6px;
  flex: 0 0 auto;
}

.lab-context__remaining {
  display: grid;
  justify-items: end;
  gap: 4px;
}

.lab-context__remaining-value {
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 1.5rem;
  font-weight: 700;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.lab-context__remaining-label,
.lab-context__closes-label {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.lab-context__closes {
  display: inline-flex;
  align-items: baseline;
  gap: 8px;
}

.lab-context__closes-value {
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.78rem;
  font-variant-numeric: tabular-nums;
}

@media (max-width: 767px) {
  .lab-context {
    align-items: stretch;
    flex-direction: column;
    gap: 14px;
  }

  .lab-context__right {
    justify-items: start;
  }

  .lab-context__remaining {
    justify-items: start;
  }
}
</style>

