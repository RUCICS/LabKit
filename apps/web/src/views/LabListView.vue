<script setup lang="ts">
import { onMounted, ref } from 'vue';
import type { PublicLab } from '../components/board/types';
import PageTitleBlock from '../components/chrome/PageTitleBlock.vue';
import LabRow from '../components/labs/LabRow.vue';

const labs = ref<PublicLab[]>([]);
const loading = ref(true);
const loadError = ref<string | null>(null);

async function loadLabs() {
  loading.value = true;
  loadError.value = null;
  try {
    const response = await fetch('/api/labs');
    if (!response.ok) {
      throw new Error(`Failed to load labs: ${response.status}`);
    }
    const payload = (await response.json()) as { labs: PublicLab[] };
    labs.value = payload.labs ?? [];
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : 'Failed to load labs';
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  void loadLabs();
});

function boardHref(labId: string) {
  return `/labs/${labId}/board`;
}
</script>

<template>
  <main class="page-shell lab-list-view" data-testid="page-shell">
    <section class="lab-list">
      <PageTitleBlock
        title="Labs"
        eyebrow="Directory"
        :lede="loading ? 'Browse public labs.' : `Browse ${labs.length} public labs.`"
      >
        <template #actions>
          <p v-if="!loading" class="lab-list__kicker">{{ labs.length }} listed</p>
        </template>
      </PageTitleBlock>

      <p v-if="loading" class="lab-list__kicker">Loading public labs…</p>
      <p v-else-if="loadError" class="lab-list__kicker">{{ loadError }}</p>
      <p v-else-if="labs.length === 0" class="lab-list__kicker">No labs available.</p>

      <ul v-else class="lab-directory">
        <li v-for="lab in labs" :key="lab.id" class="lab-directory__item">
          <LabRow :lab="lab" :to="boardHref(lab.id)" />
        </li>
      </ul>
    </section>
  </main>
</template>

<style scoped>
.lab-list-view {
  padding-top: 24px;
}

.lab-list {
  display: grid;
  gap: 18px;
}

.lab-list__kicker {
  margin: 0;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.lab-directory {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: 10px;
}

.lab-directory__item {
  list-style: none;
}

@media (max-width: 767px) {
  .lab-list-view {
    padding-top: 8px;
  }
}
</style>
