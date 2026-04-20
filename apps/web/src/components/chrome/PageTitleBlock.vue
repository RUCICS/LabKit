<script setup lang="ts">
defineProps<{
  title: string;
  eyebrow?: string;
  lede?: string;
}>();
</script>

<template>
  <header class="page-title-block">
    <div class="page-title-block__copy">
      <p v-if="eyebrow" class="page-title-block__eyebrow">
        {{ eyebrow }}
      </p>
      <h1 class="page-title-block__title">
        {{ title }}
      </h1>
      <p v-if="lede" class="page-title-block__lede">
        {{ lede }}
      </p>
    </div>

    <div v-if="$slots.actions" class="page-title-block__actions">
      <slot name="actions" />
    </div>
  </header>
</template>

<style scoped>
.page-title-block {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
}

.page-title-block__copy {
  min-width: 0;
  display: grid;
  gap: 10px;
}

.page-title-block__eyebrow {
  margin: 0;
  color: var(--accent);
  text-transform: uppercase;
  letter-spacing: 0.1em;
  font-size: 0.72rem;
  font-family: var(--font-mono);
  font-weight: 600;
}

.page-title-block__title {
  margin: 0;
  font-family: var(--font-mono);
  font-size: clamp(1.7rem, 5vw, 2.3rem);
  line-height: 0.94;
  letter-spacing: -0.06em;
}

.page-title-block__lede {
  margin: 0;
  max-width: 44rem;
  color: var(--text-secondary);
  font-size: 0.92rem;
  line-height: 1.6;
}

.page-title-block__actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  padding-top: 3px;
}

@media (max-width: 767px) {
  .page-title-block {
    flex-direction: column;
    align-items: stretch;
  }

  .page-title-block__actions {
    padding-top: 0;
  }
}
</style>
