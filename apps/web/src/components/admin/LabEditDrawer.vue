<script setup lang="ts">
import { ref, watch } from 'vue';
import { parse as parseToml } from 'smol-toml';
import { authorizedAdminHeaders, readAPIError } from '../../lib/admin';

type MetricForm = { ID: string; Name: string; Sort: string; Unit: string };
type ManifestForm = {
  newLabId: string;
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
const mode = ref<'form' | 'toml'>('form');
const tomlText = ref('');
const tomlError = ref<string | null>(null);

const newMetric = ref<MetricForm>({ ID: '', Name: '', Sort: 'desc', Unit: '' });

const form = ref<ManifestForm>({
  newLabId: '',
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
    newLabId: '',
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

function populateFormFromTomlParsed(parsed: Record<string, unknown>) {
  const lab = parsed.lab as Record<string, unknown> | undefined;
  const submit = parsed.submit as Record<string, unknown> | undefined;
  const evalSection = parsed.eval as Record<string, unknown> | undefined;
  const quota = parsed.quota as Record<string, unknown> | undefined;
  const schedule = parsed.schedule as Record<string, unknown> | undefined;
  const board = parsed.board as Record<string, unknown> | undefined;
  const rawMetrics = (parsed.metric as Array<Record<string, unknown>> | undefined) ?? [];

  function toDatetimeLocal(val: unknown): string {
    if (!val) return '';
    if (val instanceof Date) return isoToDatetimeLocal(val.toISOString());
    return isoToDatetimeLocal(String(val));
  }

  form.value = {
    newLabId: String(lab?.id ?? ''),
    labName: String(lab?.name ?? ''),
    evalImage: String(evalSection?.image ?? ''),
    evalTimeout: Number(evalSection?.timeout ?? 300),
    scheduleVisible: toDatetimeLocal(schedule?.visible),
    scheduleOpen: toDatetimeLocal(schedule?.open),
    scheduleClose: toDatetimeLocal(schedule?.close),
    quotaDaily: Number(quota?.daily ?? 0),
    submitMaxSize: String(submit?.max_size ?? '1MB'),
    submitFiles: (submit?.files as string[] | undefined) ?? [],
    metrics: rawMetrics.map((m) => ({
      ID: String(m.id ?? ''),
      Name: String(m.name ?? ''),
      Sort: String(m.sort ?? 'desc'),
      Unit: String(m.unit ?? '')
    })),
    boardRankBy: String(board?.rank_by ?? ''),
    boardPick: Boolean(board?.pick ?? false)
  };
}

function formToToml(f: ManifestForm): string {
  const id = props.labId ?? f.newLabId;
  const lines: string[] = [];

  lines.push('[lab]');
  lines.push(`id = ${tomlStr(id)}`);
  lines.push(`name = ${tomlStr(f.labName)}`);
  lines.push('');

  lines.push('[submit]');
  lines.push(`files = [${f.submitFiles.map(tomlStr).join(', ')}]`);
  lines.push(`max_size = ${tomlStr(f.submitMaxSize)}`);
  lines.push('');

  lines.push('[eval]');
  lines.push(`image = ${tomlStr(f.evalImage)}`);
  lines.push(`timeout = ${f.evalTimeout}`);
  lines.push('');

  lines.push('[quota]');
  lines.push(`daily = ${f.quotaDaily}`);
  lines.push('');

  for (const m of f.metrics) {
    lines.push('[[metric]]');
    lines.push(`id = ${tomlStr(m.ID)}`);
    lines.push(`name = ${tomlStr(m.Name)}`);
    lines.push(`sort = ${tomlStr(m.Sort)}`);
    lines.push(`unit = ${tomlStr(m.Unit)}`);
    lines.push('');
  }

  lines.push('[board]');
  lines.push(`rank_by = ${tomlStr(f.boardRankBy)}`);
  lines.push(`pick = ${f.boardPick}`);
  lines.push('');

  lines.push('[schedule]');
  if (f.scheduleVisible) lines.push(`visible = ${datetimeLocalToISO(f.scheduleVisible)}`);
  if (f.scheduleOpen) lines.push(`open = ${datetimeLocalToISO(f.scheduleOpen)}`);
  if (f.scheduleClose) lines.push(`close = ${datetimeLocalToISO(f.scheduleClose)}`);

  return lines.join('\n').trimEnd() + '\n';
}

function tomlStr(s: string): string {
  return JSON.stringify(s);
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
    tomlText.value = formToToml(form.value);
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
    tomlError.value = null;
    mode.value = 'form';
    if (labId) {
      void fetchLab(labId);
    } else {
      form.value = {
        newLabId: '', labName: '', evalImage: '', evalTimeout: 300,
        scheduleVisible: '', scheduleOpen: '', scheduleClose: '',
        quotaDaily: 0, submitMaxSize: '1MB', submitFiles: [],
        metrics: [], boardRankBy: '', boardPick: false
      };
      tomlText.value = formToToml(form.value);
    }
  },
  { immediate: true }
);

function switchToToml() {
  tomlText.value = formToToml(form.value);
  mode.value = 'toml';
}

function switchToForm() {
  tomlError.value = null;
  try {
    const parsed = parseToml(tomlText.value) as Record<string, unknown>;
    populateFormFromTomlParsed(parsed);
    mode.value = 'form';
  } catch (e) {
    tomlError.value = e instanceof Error ? e.message : 'Invalid TOML.';
  }
}

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

function addMetric() {
  const m = newMetric.value;
  if (!m.ID.trim()) return;
  if (form.value.metrics.some((x) => x.ID === m.ID.trim())) return;
  form.value.metrics = [
    ...form.value.metrics,
    { ID: m.ID.trim(), Name: m.Name.trim() || m.ID.trim(), Sort: m.Sort, Unit: m.Unit.trim() }
  ];
  if (!form.value.boardRankBy) form.value.boardRankBy = m.ID.trim();
  newMetric.value = { ID: '', Name: '', Sort: 'desc', Unit: '' };
}

function removeMetric(index: number) {
  const removed = form.value.metrics[index];
  form.value.metrics = form.value.metrics.filter((_, i) => i !== index);
  if (form.value.boardRankBy === removed.ID) {
    form.value.boardRankBy = form.value.metrics[0]?.ID ?? '';
  }
}

function buildJsonPayload() {
  const f = form.value;
  const id = props.labId ?? f.newLabId;
  return JSON.stringify({
    Lab: { ID: id, Name: f.labName, Tags: {} },
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
  saveError.value = '';
  const isNew = !props.labId;
  const body = mode.value === 'toml' ? tomlText.value : buildJsonPayload();
  const contentType = mode.value === 'toml' ? 'text/plain' : 'application/json';
  const url = isNew
    ? '/api/admin/labs'
    : `/api/admin/labs/${encodeURIComponent(props.labId!)}`;
  const method = isNew ? 'POST' : 'PUT';

  saving.value = true;
  try {
    const res = await fetch(url, {
      method,
      headers: authorizedAdminHeaders({ 'Content-Type': contentType }),
      body
    });
    if (!res.ok) throw new Error(await readAPIError(res, 'Failed to save lab.'));
    const data = await res.json() as { id?: string };
    emit('saved', data.id ?? props.labId ?? '');
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

          <!-- Mode tabs -->
          <div class="drawer__tabs">
            <button
              type="button"
              class="drawer__tab"
              :class="{ 'drawer__tab--active': mode === 'form' }"
              @click="mode === 'toml' ? switchToForm() : undefined"
            >Form</button>
            <button
              type="button"
              class="drawer__tab"
              :class="{ 'drawer__tab--active': mode === 'toml' }"
              @click="mode === 'form' ? switchToToml() : undefined"
            >TOML</button>
          </div>
          <p v-if="tomlError" class="drawer__toml-error">{{ tomlError }}</p>

          <div v-if="loading" class="drawer__loading">Loading…</div>
          <div v-else-if="error" class="drawer__error">
            {{ error }}
            <button type="button" class="button button--secondary" @click="labId && fetchLab(labId)">Retry</button>
          </div>

          <!-- TOML mode -->
          <div v-else-if="mode === 'toml'" class="drawer__body drawer__body--toml">
            <textarea
              v-model="tomlText"
              class="toml-editor"
              spellcheck="false"
              autocorrect="off"
              autocapitalize="off"
            />
          </div>

          <!-- Form mode -->
          <div v-else class="drawer__body">
            <!-- LAB -->
            <div class="form-group">
              <div class="form-group__label">LAB</div>
              <div class="form-grid form-grid--2">
                <label v-if="!labId" class="field field--stacked" style="grid-column:1/-1">
                  <span>Lab ID <span class="field__required">*</span></span>
                  <input v-model="form.newLabId" type="text" placeholder="e.g. lab-2024-spring" />
                </label>
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
                    <span v-for="f in form.submitFiles" :key="f" class="tag">
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

            <!-- METRICS -->
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
                <button type="button" class="metric-row__remove" @click="removeMetric(i)" title="Remove metric">✕</button>
              </div>

              <!-- Add new metric -->
              <form class="metric-add" @submit.prevent="addMetric">
                <input v-model="newMetric.ID" type="text" placeholder="id" class="metric-add__id" />
                <input v-model="newMetric.Name" type="text" placeholder="name" class="metric-add__name" />
                <select v-model="newMetric.Sort" class="metric-add__sort">
                  <option value="asc">asc</option>
                  <option value="desc">desc</option>
                </select>
                <input v-model="newMetric.Unit" type="text" placeholder="unit" class="metric-add__unit" />
                <button type="submit" class="metric-add__btn">+ add</button>
              </form>
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
              >{{ saving ? 'Saving…' : (labId ? 'Save changes' : 'Create lab') }}</button>
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

.drawer__tabs {
  display: flex;
  border-bottom: 1px solid var(--border-default);
  flex-shrink: 0;
}

.drawer__tab {
  background: none;
  border: none;
  padding: 8px 20px;
  font-size: 0.8rem;
  font-weight: 500;
  color: var(--text-secondary);
  cursor: pointer;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
}

.drawer__tab--active {
  color: var(--accent-strong);
  border-bottom-color: var(--accent-strong);
}

.drawer__tab:hover:not(.drawer__tab--active) {
  color: var(--text-primary);
}

.drawer__toml-error {
  color: var(--danger);
  font-size: 0.8rem;
  margin: 6px 20px 0;
  flex-shrink: 0;
}

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

.drawer__body--toml {
  padding: 0;
}

.toml-editor {
  flex: 1;
  width: 100%;
  height: 100%;
  resize: none;
  font-family: var(--font-mono);
  font-size: 0.82rem;
  line-height: 1.6;
  padding: 16px 20px;
  border: none;
  outline: none;
  background: var(--bg-default);
  color: var(--text-primary);
  box-sizing: border-box;
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
.metric-row__id { font-family: var(--font-mono); font-size: 0.82rem; font-weight: 600; min-width: 80px; }
.metric-row__remove {
  background: none;
  border: none;
  color: var(--text-tertiary);
  cursor: pointer;
  font-size: 0.75rem;
  padding: 2px 4px;
  margin-left: auto;
  opacity: 0.6;
}
.metric-row__remove:hover { opacity: 1; color: var(--danger); }

.metric-add {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 0 4px;
  flex-wrap: wrap;
}
.metric-add__id { width: 80px; }
.metric-add__name { flex: 1; min-width: 80px; }
.metric-add__sort { width: 60px; }
.metric-add__unit { width: 60px; }
.metric-add input, .metric-add select {
  padding: 3px 6px;
  font-size: 0.78rem;
  border: 1px dashed var(--border-default);
  border-radius: 4px;
  background: transparent;
  color: var(--text-primary);
}
.metric-add__btn {
  background: none;
  border: none;
  color: var(--text-tertiary);
  font-size: 0.78rem;
  cursor: pointer;
  white-space: nowrap;
}
.metric-add__btn:hover { color: var(--text-primary); }

.field--inline { display: flex; align-items: center; gap: 4px; font-size: 0.78rem; }
.field--inline span { color: var(--text-tertiary); white-space: nowrap; }
.field--inline input, .field--inline select { padding: 3px 6px; font-size: 0.78rem; }

.field__required { color: var(--danger); }

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
