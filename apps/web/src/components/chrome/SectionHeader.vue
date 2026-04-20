<script setup lang="ts">
defineProps<{
  title: string;
  eyebrow?: string;
  subtitle?: string;
}>();
</script>

<template>
  <header class="section-header">
    <div class="section-header__copy">
      <p v-if="eyebrow" class="section-header__eyebrow">
        {{ eyebrow }}
      </p>
      <h2 class="section-header__title">
        {{ title }}
      </h2>
      <p v-if="subtitle" class="section-header__subtitle">
        {{ subtitle }}
      </p>
    </div>

    <div v-if="$slots.actions" class="section-header__actions">
      <slot name="actions" />
    </div>
  </header>
</template>

<style scoped>
.section-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.section-header__copy {
  min-width: 0;
  display: grid;
  gap: 8px;
}

.section-header__eyebrow {
  margin: 0;
  color: var(--text-tertiary);
  text-transform: uppercase;
  letter-spacing: 0.1em;
  font-size: 0.7rem;
  font-family: var(--font-mono);
  font-weight: 600;
}

.section-header__title {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 0.95rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.section-header__subtitle {
  margin: 0;
  color: var(--text-secondary);
  font-size: 0.9rem;
  line-height: 1.6;
}

.section-header__actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  padding-top: 1px;
}

@media (max-width: 767px) {
  .section-header {
    flex-direction: column;
    align-items: stretch;
  }

  .section-header__actions {
    padding-top: 0;
  }
}
</style>
