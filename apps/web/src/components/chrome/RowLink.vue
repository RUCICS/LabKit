<script setup lang="ts">
import { computed } from 'vue';
import { RouterLink } from 'vue-router';
import type { RouteLocationRaw } from 'vue-router';

const props = defineProps<{
  to?: RouteLocationRaw;
  href?: string;
  target?: string;
  rel?: string;
  ariaLabel?: string;
  disabled?: boolean;
  tone?: 'default' | 'muted';
}>();

const isEffectivelyDisabled = computed(() => props.disabled || (!props.to && !props.href));

const tag = computed(() => {
  if (isEffectivelyDisabled.value) return 'span';
  if (props.to) return RouterLink;
  return 'a';
});

const linkAttrs = computed(() => {
  if (tag.value === RouterLink) {
    return { to: props.to } as const;
  }

  if (tag.value === 'a') {
    const target = props.target;
    const rel =
      target === '_blank'
        ? [props.rel, 'noopener', 'noreferrer'].filter(Boolean).join(' ')
        : props.rel;

    return {
      href: props.href,
      target,
      rel
    } as const;
  }

  return {
    'aria-disabled': 'true'
  } as const;
});
</script>

<template>
  <component
    :is="tag"
    class="lk-row-link row-link"
    :data-tone="tone ?? 'default'"
    :aria-label="ariaLabel"
    v-bind="linkAttrs"
  >
    <span class="row-link__content">
      <span class="row-link__title">
        <slot />
      </span>
      <span v-if="$slots.subtitle" class="row-link__subtitle">
        <slot name="subtitle" />
      </span>
    </span>

    <span v-if="$slots.meta" class="row-link__meta">
      <slot name="meta" />
    </span>
  </component>
</template>

<style scoped>
.row-link {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 14px;
  min-height: 44px;
  padding: 12px 14px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
  text-decoration: none;
}

.row-link[data-tone='muted'] {
  color: var(--text-secondary);
}

.row-link__content {
  min-width: 0;
  display: grid;
  gap: 4px;
  flex: 1 1 14rem;
}

.row-link__title {
  min-width: 0;
  font-family: var(--font-mono);
  font-size: 0.8rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.row-link__subtitle {
  min-width: 0;
  color: var(--text-secondary);
  font-size: 0.9rem;
  line-height: 1.55;
  text-transform: none;
  letter-spacing: normal;
}

.row-link__meta {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  flex: 0 1 auto;
}

@media (max-width: 480px) {
  .row-link {
    align-items: flex-start;
  }

  .row-link__meta {
    width: 100%;
    margin-left: 0;
    justify-content: flex-start;
  }
}
</style>
