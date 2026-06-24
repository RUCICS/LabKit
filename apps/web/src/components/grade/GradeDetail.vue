<script setup lang="ts">
import { computed } from 'vue';
import type { FinalGrade } from '../../lib/grade';
import SectionHeader from '../chrome/SectionHeader.vue';

const props = defineProps<{ grade: FinalGrade }>();

const hasTotal = computed(() => Boolean(props.grade.total && props.grade.total.trim()));
const items = computed(() => props.grade.items ?? []);

function formatUpdatedAt(value: string | undefined) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(date);
}
</script>

<template>
  <div class="grade-detail">
    <section v-if="hasTotal" class="grade-detail__panel grade-detail__total">
      <SectionHeader title="成绩" subtitle="Total" />
      <p class="grade-detail__total-value" data-testid="grade-total">{{ grade.total }}</p>
    </section>

    <section v-if="items.length" class="grade-detail__panel">
      <SectionHeader title="分项" subtitle="Breakdown" />
      <dl class="grade-detail__grid">
        <div v-for="(item, i) in items" :key="`${item.label}-${i}`" class="grade-detail__cell">
          <dt>{{ item.label }}</dt>
          <dd>{{ item.value }}</dd>
        </div>
      </dl>
    </section>

    <section v-if="!hasTotal && !items.length" class="grade-detail__panel">
      <p class="grade-detail__status">暂无成绩明细。</p>
    </section>

    <section v-if="grade.remark && grade.remark.trim()" class="grade-detail__panel">
      <SectionHeader title="备注" subtitle="Remark" />
      <p class="grade-detail__remark">{{ grade.remark }}</p>
    </section>

    <p v-if="grade.updated_at" class="grade-detail__meta">
      更新于 {{ formatUpdatedAt(grade.updated_at) }}
    </p>
  </div>
</template>

<style scoped>
.grade-detail {
  display: grid;
  gap: 16px;
}

.grade-detail__panel {
  display: grid;
  gap: 14px;
  padding: 24px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-surface);
}

.grade-detail__status {
  margin: 0;
  color: var(--text-secondary);
  line-height: 1.55;
}

.grade-detail__total {
  text-align: center;
}

.grade-detail__total-value {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 3rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  color: var(--text-primary);
}

.grade-detail__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
  margin: 0;
}

.grade-detail__cell {
  display: grid;
  gap: 6px;
  padding: 14px;
  border: 1px solid var(--border-default);
  border-radius: 10px;
  background: var(--bg-elevated);
}

.grade-detail__cell dt {
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.7rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.grade-detail__cell dd {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--text-primary);
}

.grade-detail__remark {
  margin: 0;
  color: var(--text-secondary);
  line-height: 1.6;
  white-space: pre-wrap;
}

.grade-detail__meta {
  margin: 0;
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.74rem;
}
</style>
