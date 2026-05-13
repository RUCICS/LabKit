<script setup lang="ts">
import { ref, watch } from 'vue';
import { authorizedAdminHeaders, readAPIError } from '../../lib/admin';

type MetricForm = { ID: string; Name: string; Sort: string; Unit: string };
type ManifestForm = {
  labName: string;
  evalImage: string;
  evalTimeout: number;
  scheduleVisible: string;
  scheduleOpen: string;
  scheduleClose: string;
  quotaDaily: number;
  submitMaxSize: string;
  submitFiles: string[];
  metrics: MetricForm[];
  boardRankBy: string;
  boardPick: boolean;
};

const props = defineProps<{
  labId: string | null;
  open: boolean;
}>();

const emit = defineEmits<{
  (e: 'close'): void;
  (e: 'saved', labId: string): void;
}>();

const loading = ref(false);
const error = ref<string | null>(null);
const saving = ref(false);
const saveError = ref<string | null>(null);
const newFileInput = ref('');

const form = ref<ManifestForm>({
  labName: '',
  evalImage: '',
  evalTimeout: 300,
  scheduleVisible: '',
  scheduleOpen: '',
  scheduleClose: '',
  quotaDaily: 0,
  submitMaxSize: '1MB',
  submitFiles: [],
  metrics: [],
  boardRankBy: '',
  boardPick: false
});

function isoToDatetimeLocal(iso: string): string {
  if (!iso || iso.startsWith('0001')) return '';
  try {
    return new Date(iso).toISOString().slice(0, 16);
  } catch {
    return '';
  }
}

function datetimeLocalToISO(local: string): string {
  if (!local) return '0001-01-01T00:00:00Z';
  return new Date(local).toISOString();
}

function populateForm(data: Record<string, unknown>) {
  const m = data.manifest as Record<string, unknown> | undefined;
  if (!m) return;
  const lab = m.Lab as Record<string, unknown> | undefined;
  const submit = m.Submit as Record<string, unknown> | undefined;
  const evalSection = m.Eval as Record<string, unknown> | undefined;
  const quota = m.Quota as Record<string, unknown> | undefined;
  const schedule = m.Schedule as Record<string, unknown> | undefined;
  const board = m.Board as Record<string, unknown> | undefined;
  const metrics = (m.Metrics as MetricForm[] | undefined) ?? [];

  form.value = {
    labName: String(lab?.Name ?? ''),
    evalImage: String(evalSection?.Image ?? ''),
    evalTimeout: Number(evalSection?.Timeout ?? 300),
    scheduleVisible: isoToDatetimeLocal(String(schedule?.Visible ?? '')),
    scheduleOpen: isoToDatetimeLocal(String(schedule?.Open ?? '')),
    scheduleClose: isoToDatetimeLocal(String(schedule?.Close ?? '')),
    quotaDaily: Number(quota?.Daily ?? 0),
    submitMaxSize: String(submit?.MaxSize ?? '1MB'),
    submitFiles: (submit?.Files as string[] | undefined) ?? [],
    metrics: metrics.map((m) => ({ ID: m.ID, Name: m.Name, Sort: m.Sort, Unit: m.Unit })),
    boardRankBy: String(board?.RankBy ?? ''),
    boardPick: Boolean(board?.Pick ?? false)
  };
}

async function fetchLab(labId: string) {
  loading.value = true;
  error.value = null;
  try {
    const res = await fetch(`/api/admin/labs/${encodeURIComponent(labId)}`, {
      headers: authorizedAdminHeaders()
    });
    if (!res.ok) throw new Error(await readAPIError(res, 'Failed to load lab.'));
    const data = await res.json() as Record<string, unknown>;
    populateForm(data);
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load lab.';
  } finally {
    loading.value = false;
  }
}

watch(
  () => [props.open, props.labId] as const,
  ([open, labId]) => {
    if (!open) return;
    saveError.value = null;
    if (labId) {
      void fetchLab(labId);
    } else {
      form.value = {
        labName: '', evalImage: '', evalTimeout: 300,
        scheduleVisible: '', scheduleOpen: '', scheduleClose: '',
        quotaDaily: 0, submitMaxSize: '1MB', submitFiles: [],
        metrics: [], boardRankBy: '', boardPick: false
      };
    }
  },
  { immediate: true }
);

function addFile() {
  const f = newFileInput.value.trim();
  if (f && !form.value.submitFiles.includes(f)) {
    form.value.submitFiles = [...form.value.submitFiles, f];
  }
  newFileInput.value = '';
}

function removeFile(name: string) {
  form.value.submitFiles = form.value.submitFiles.filter((f) => f !== name);
}

function buildPayload() {
  const f = form.value;
  return JSON.stringify({
    Lab: { ID: props.labId ?? '', Name: f.labName, Tags: {} },
    Submit: { Files: f.submitFiles, MaxSize: f.submitMaxSize },
    Eval: { Image: f.evalImage, Timeout: f.evalTimeout },
    Quota: { Daily: f.quotaDaily, Free: [] },
    Metrics: f.metrics,
    Board: { RankBy: f.boardRankBy, Pick: f.boardPick },
    Schedule: {
      Visible: datetimeLocalToISO(f.scheduleVisible),
      Open: datetimeLocalToISO(f.scheduleOpen),
      Close: datetimeLocalToISO(f.scheduleClose)
    }
  });
}

async function save() {
  if (!props.labId) return;
  saving.value = true;
  saveError.value = null;
  try {
    const res = await fetch(`/api/admin/labs/${encodeURIComponent(props.labId)}`, {
      method: 'PUT',
      headers: authorizedAdminHeaders({ 'Content-Type': 'application/json' }),
      body: buildPayload()
    });
    if (!res.ok) throw new Error(await readAPIError(res, 'Failed to save lab.'));
    emit('saved', props.labId);
  } catch (e) {
    saveError.value = e instanceof Error ? e.message : 'Failed to save lab.';
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <Teleport to="body">
    <Transition name="drawer">
      <div v-if="open" class="drawer-overlay" data-testid="lab-edit-drawer" @click.self="emit('close')">
        <aside class="drawer">
          <div class="drawer__head">
            <div>
              <div class="drawer__title">{{ labId ? `Edit: ${labId}` : 'New lab' }}</div>
              <div v-if="labId" class="drawer__sub">{{ labId }}</div>
            </div>
            <button type="button" class="drawer__close" @click="emit('close')">✕</button>
          </div>

          <div v-if="loading" class="drawer__loading">Loading…</div>
          <div v-else-if="error" class="drawer__error">
            {{ error }}
            <button type="button" class="button button--secondary" @click="labId && fetchLab(labId)">Retry</button>
          </div>
          <div v-else class="drawer__body">
            <!-- LAB -->
            <div class="form-group">
              <div class="form-group__label">LAB</div>
              <div class="form-grid form-grid--2">
                <label class="field field--stacked" style="grid-column:1/-1">
                  <span>Name</span>
                  <input v-model="form.labName" type="text" />
                </label>
                <label class="field field--stacked">
                  <span>Eval Image</span>
                  <input v-model="form.evalImage" type="text" placeholder="registry/image:tag" />
                </label>
                <label class="field field--stacked">
                  <span>Timeout (s)</span>
                  <input v-model.number="form.evalTimeout" type="number" min="1" />
                </label>
              </div>
            </div>

            <!-- SCHEDULE -->
            <div class="form-group">
              <div class="form-group__label">SCHEDULE</div>
              <div class="form-grid form-grid--3">
                <label class="field field--stacked">
                  <span>Visible</span>
                  <input v-model="form.scheduleVisible" type="datetime-local" />
                </label>
                <label class="field field--stacked">
                  <span>Open</span>
                  <input v-model="form.scheduleOpen" type="datetime-local" />
                </label>
                <label class="field field--stacked">
                  <span>Close</span>
                  <input v-model="form.scheduleClose" type="datetime-local" />
                </label>
              </div>
            </div>

            <!-- QUOTA & FILES -->
            <div class="form-group">
              <div class="form-group__label">QUOTA & FILES</div>
              <div class="form-grid form-grid--2">
                <label class="field field--stacked">
                  <span>Daily limit</span>
                  <input v-model.number="form.quotaDaily" type="number" min="0" />
                </label>
                <label class="field field--stacked">
                  <span>Max size</span>
                  <input v-model="form.submitMaxSize" type="text" placeholder="1MB" />
                </label>
                <div class="field field--stacked" style="grid-column:1/-1">
                  <span>Submit files</span>
                  <div class="tag-list">
                    <span
                      v-for="f in form.submitFiles"
                      :key="f"
                      class="tag"
                    >
                      {{ f }}
                      <button type="button" class="tag__remove" @click="removeFile(f)">✕</button>
                    </span>
                    <form class="tag-add" @submit.prevent="addFile">
                      <input v-model="newFileInput" type="text" placeholder="filename.c" class="tag-add__input" />
                      <button type="submit" class="tag-add__btn">+ add</button>
                    </form>
                  </div>
                </div>
              </div>
            </div>

            <!-- METRICS (read-only structure, editable display) -->
            <div class="form-group">
              <div class="form-group__label">METRICS</div>
              <div v-for="(m, i) in form.metrics" :key="m.ID" class="metric-row">
                <span class="metric-row__id">{{ m.ID }}</span>
                <label class="field field--inline">
                  <span>Name</span>
                  <input v-model="form.metrics[i].Name" type="text" />
                </label>
                <label class="field field--inline">
                  <span>Sort</span>
                  <select v-model="form.metrics[i].Sort">
                    <option value="asc">asc</option>
                    <option value="desc">desc</option>
                  </select>
                </label>
                <label class="field field--inline">
                  <span>Unit</span>
                  <input v-model="form.metrics[i].Unit" type="text" style="width:60px" />
                </label>
              </div>
              <p v-if="form.metrics.length === 0" class="drawer__empty">No metrics</p>
            </div>

            <!-- BOARD -->
            <div class="form-group">
              <div class="form-group__label">BOARD</div>
              <div class="form-grid form-grid--2">
                <label class="field field--stacked">
                  <span>Rank by</span>
                  <select v-model="form.boardRankBy">
                    <option v-for="m in form.metrics" :key="m.ID" :value="m.ID">{{ m.ID }}</option>
                  </select>
                </label>
                <label class="field field--stacked">
                  <span>Pick track</span>
                  <select v-model="form.boardPick">
                    <option :value="false">off</option>
                    <option :value="true">on</option>
                  </select>
                </label>
              </div>
            </div>
          </div>

          <div class="drawer__footer">
            <p v-if="saveError" class="drawer__save-error">{{ saveError }}</p>
            <div class="drawer__footer-actions">
              <button type="button" class="button button--secondary" @click="emit('close')">Cancel</button>
              <button
                type="button"
                class="button"
                :disabled="saving || loading"
                @click="save"
              >{{ saving ? 'Saving…' : 'Save changes' }}</button>
            </div>
          </div>
        </aside>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.drawer-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  justify-content: flex-end;
  z-index: 200;
}

.drawer {
  width: min(520px, 90vw);
  height: 100%;
  background: var(--bg-surface);
  border-left: 1px solid var(--border-default);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.drawer__head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 20px 20px 16px;
  border-bottom: 1px solid var(--border-default);
  flex-shrink: 0;
}

.drawer__title { font-size: 1rem; font-weight: 700; margin: 0; }
.drawer__sub { color: var(--text-tertiary); font-size: 0.75rem; font-family: var(--font-mono); margin-top: 2px; }
.drawer__close { background: none; border: none; color: var(--text-tertiary); font-size: 1.1rem; cursor: pointer; padding: 0; }

.drawer__loading, .drawer__error {
  padding: 24px 20px;
  color: var(--text-secondary);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.drawer__error { color: var(--danger); }

.drawer__body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 0;
}

.drawer__footer {
  padding: 14px 20px;
  border-top: 1px solid var(--border-default);
  flex-shrink: 0;
}

.drawer__footer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.drawer__save-error { color: var(--danger); font-size: 0.82rem; margin: 0 0 8px; }
.drawer__empty { color: var(--text-tertiary); font-size: 0.82rem; margin: 0; }

.form-group { margin-bottom: 20px; }
.form-group__label {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.1em;
  margin-bottom: 8px;
}

.form-grid { display: grid; gap: 10px; }
.form-grid--2 { grid-template-columns: 1fr 1fr; }
.form-grid--3 { grid-template-columns: 1fr 1fr 1fr; }

.tag-list { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; margin-top: 6px; }
.tag {
  background: color-mix(in srgb, var(--accent) 15%, transparent);
  color: var(--accent-strong);
  font-size: 0.78rem;
  padding: 3px 8px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  gap: 4px;
}
.tag__remove { background: none; border: none; color: inherit; cursor: pointer; padding: 0; font-size: 0.7rem; opacity: 0.6; }
.tag__remove:hover { opacity: 1; }
.tag-add { display: flex; gap: 4px; align-items: center; }
.tag-add__input { width: 100px; padding: 3px 6px; font-size: 0.78rem; border: 1px dashed var(--border-default); border-radius: 12px; background: transparent; color: var(--text-primary); }
.tag-add__btn { background: none; border: none; color: var(--text-tertiary); font-size: 0.78rem; cursor: pointer; }
.tag-add__btn:hover { color: var(--text-primary); }

.metric-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-default);
  flex-wrap: wrap;
}
.metric-row:last-child { border-bottom: none; }
.metric-row__id { font-family: var(--font-mono); font-size: 0.82rem; font-weight: 600; min-width: 80px; }

.field--inline { display: flex; align-items: center; gap: 4px; font-size: 0.78rem; }
.field--inline span { color: var(--text-tertiary); white-space: nowrap; }
.field--inline input, .field--inline select { padding: 3px 6px; font-size: 0.78rem; }

.drawer-enter-active,
.drawer-leave-active {
  transition: opacity 150ms ease;
}
.drawer-enter-active .drawer,
.drawer-leave-active .drawer {
  transition: transform 150ms ease;
}
.drawer-enter-from,
.drawer-leave-to {
  opacity: 0;
}
.drawer-enter-from .drawer,
.drawer-leave-to .drawer {
  transform: translateX(100%);
}
</style>
