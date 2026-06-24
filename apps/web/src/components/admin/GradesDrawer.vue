<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { authorizedAdminHeaders, readAPIError } from '../../lib/admin';
import { previewGradeCsv, type GradeCsvPreview } from '../../lib/csv';

type GradeStatus = {
  lab_id: string;
  total: number;
  published: number;
  unpublished: number;
  last_updated_at?: string;
};

const props = defineProps<{
  labId: string | null;
  labName: string;
  open: boolean;
}>();

const emit = defineEmits<{ (e: 'close'): void }>();

const statusLoading = ref(false);
const statusError = ref<string | null>(null);
const status = ref<GradeStatus | null>(null);

const dragActive = ref(false);
const file = ref<File | null>(null);
const preview = ref<GradeCsvPreview | null>(null);
const previewError = ref<string | null>(null);

const importing = ref(false);
const importError = ref<string | null>(null);
const importResult = ref<string | null>(null);

const publishConfirm = ref(false);
const publishing = ref(false);
const publishError = ref<string | null>(null);

const clearConfirm = ref(false);
const clearing = ref(false);
const clearError = ref<string | null>(null);

const busy = computed(() => importing.value || publishing.value || clearing.value);

function base(): string {
  return `/api/admin/labs/${encodeURIComponent(props.labId ?? '')}/grades`;
}

const missingColumns = computed(() => {
  if (!preview.value) return [] as string[];
  const missing: string[] = [];
  if (!preview.value.hasStudentId) missing.push('student_id');
  if (!preview.value.hasTotal) missing.push('total');
  return missing;
});

const canImport = computed(
  () =>
    Boolean(file.value) &&
    Boolean(preview.value) &&
    missingColumns.value.length === 0 &&
    (preview.value?.dataRowCount ?? 0) > 0 &&
    !importing.value
);

watch(
  () => [props.open, props.labId] as const,
  ([open]) => {
    if (!open) return;
    resetState();
    void loadStatus();
  },
  { immediate: true }
);

function resetState() {
  statusError.value = null;
  status.value = null;
  dragActive.value = false;
  file.value = null;
  preview.value = null;
  previewError.value = null;
  importError.value = null;
  importResult.value = null;
  publishConfirm.value = false;
  publishError.value = null;
  clearConfirm.value = false;
  clearError.value = null;
}

async function loadStatus() {
  if (!props.labId) return;
  statusLoading.value = true;
  statusError.value = null;
  try {
    const res = await fetch(`${base()}/status`, { headers: authorizedAdminHeaders() });
    if (!res.ok) throw new Error(await readAPIError(res, 'Failed to load grade status.'));
    status.value = (await res.json()) as GradeStatus;
  } catch (e) {
    statusError.value = e instanceof Error ? e.message : 'Failed to load grade status.';
  } finally {
    statusLoading.value = false;
  }
}

async function onFilePicked(picked: File | null) {
  importResult.value = null;
  importError.value = null;
  previewError.value = null;
  preview.value = null;
  file.value = picked;
  if (!picked) return;
  try {
    const text = await readFileText(picked);
    preview.value = previewGradeCsv(text);
  } catch {
    previewError.value = '无法读取该文件。';
    file.value = null;
  }
}

// Blob.text() in modern browsers; FileReader fallback for environments (e.g.
// jsdom) that don't implement it.
function readFileText(target: File): Promise<string> {
  if (typeof target.text === 'function') {
    return target.text();
  }
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result ?? ''));
    reader.onerror = () => reject(reader.error ?? new Error('read failed'));
    reader.readAsText(target);
  });
}

function onInputChange(event: Event) {
  const input = event.target as HTMLInputElement;
  void onFilePicked(input.files?.[0] ?? null);
}

function onDrop(event: DragEvent) {
  dragActive.value = false;
  void onFilePicked(event.dataTransfer?.files?.[0] ?? null);
}

function clearFile() {
  file.value = null;
  preview.value = null;
  previewError.value = null;
}

async function doImport() {
  if (!props.labId || !file.value || !canImport.value) return;
  importing.value = true;
  importError.value = null;
  importResult.value = null;
  try {
    const form = new FormData();
    form.append('file', file.value);
    const res = await fetch(`${base()}/import`, {
      method: 'POST',
      headers: authorizedAdminHeaders(),
      body: form
    });
    if (!res.ok) throw new Error(await readAPIError(res, 'Import failed.'));
    const data = (await res.json()) as { imported?: number };
    importResult.value = `已导入 ${data.imported ?? 0} 条,尚未发布`;
    clearFile();
    await loadStatus();
  } catch (e) {
    importError.value = e instanceof Error ? e.message : 'Import failed.';
  } finally {
    importing.value = false;
  }
}

async function doPublish() {
  if (!props.labId) return;
  publishing.value = true;
  publishError.value = null;
  try {
    const res = await fetch(`${base()}/publish`, { method: 'POST', headers: authorizedAdminHeaders() });
    if (!res.ok) throw new Error(await readAPIError(res, 'Publish failed.'));
    publishConfirm.value = false;
    await loadStatus();
  } catch (e) {
    publishError.value = e instanceof Error ? e.message : 'Publish failed.';
  } finally {
    publishing.value = false;
  }
}

async function doClear() {
  if (!props.labId) return;
  clearing.value = true;
  clearError.value = null;
  try {
    const res = await fetch(base(), { method: 'DELETE', headers: authorizedAdminHeaders() });
    if (!res.ok) throw new Error(await readAPIError(res, 'Clear failed.'));
    clearConfirm.value = false;
    importResult.value = null;
    await loadStatus();
  } catch (e) {
    clearError.value = e instanceof Error ? e.message : 'Clear failed.';
  } finally {
    clearing.value = false;
  }
}

function downloadTemplate() {
  const csv =
    [
      'student_id,track,ratio,perf_score,percentile,board_score,total,remark',
      '2026001,throughput,1.2,85,0.91,14,86.5,',
      '2026002,latency,1.0,70,0.5,10,72,申诉后 +2'
    ].join('\n') + '\n';
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = 'grades-template.csv';
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function formatUpdated(iso?: string): string {
  if (!iso) return '—';
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(date);
}

function onClose() {
  if (busy.value) return;
  emit('close');
}
</script>

<template>
  <Teleport to="body">
    <Transition name="drawer">
      <div v-if="open" class="drawer-overlay" data-testid="grades-drawer" @click.self="onClose">
        <aside class="drawer">
          <div class="drawer__head">
            <div>
              <div class="drawer__title">成绩管理</div>
              <div class="drawer__sub">{{ labName || labId }} · {{ labId }}</div>
            </div>
            <button type="button" class="drawer__close" :disabled="busy" @click="onClose">✕</button>
          </div>

          <div class="drawer__body">
            <!-- STATUS -->
            <section class="section">
              <div class="section__label">当前状态</div>
              <p v-if="statusLoading" class="section__muted">加载中…</p>
              <p v-else-if="statusError" class="section__error">{{ statusError }}</p>
              <template v-else-if="status">
                <div class="stats">
                  <div class="stat">
                    <span class="stat__num" data-testid="grades-total">{{ status.total }}</span>
                    <span class="stat__label">已导入</span>
                  </div>
                  <div class="stat">
                    <span class="stat__num stat__num--ok" data-testid="grades-published">{{ status.published }}</span>
                    <span class="stat__label">已发布</span>
                  </div>
                  <div class="stat">
                    <span class="stat__num stat__num--pending" data-testid="grades-pending">{{ status.unpublished }}</span>
                    <span class="stat__label">待发布</span>
                  </div>
                </div>
                <p class="section__hint">
                  {{ status.total === 0 ? '尚未导入任何成绩。' : `最近更新:${formatUpdated(status.last_updated_at)}` }}
                </p>
              </template>
            </section>

            <!-- UPLOAD -->
            <section class="section">
              <div class="section__label">导入 CSV</div>
              <label
                class="drop"
                :class="{ 'drop--active': dragActive }"
                @dragover.prevent="dragActive = true"
                @dragleave.prevent="dragActive = false"
                @drop.prevent="onDrop"
              >
                <input
                  type="file"
                  accept=".csv,text/csv"
                  class="drop__input"
                  data-testid="grades-file"
                  @change="onInputChange"
                />
                <span class="drop__icon" aria-hidden="true">⬆</span>
                <span class="drop__text">
                  <strong>{{ file ? file.name : '拖入 CSV 或点击选择文件' }}</strong>
                  <span class="drop__sub">按列名匹配,需包含 student_id 与 total</span>
                </span>
              </label>

              <p v-if="previewError" class="section__error">{{ previewError }}</p>

              <div v-if="preview" class="preview" data-testid="grades-preview">
                <div class="preview__cols">
                  <span
                    v-for="col in preview.columns"
                    :key="col"
                    class="chip"
                    :class="{ 'chip--key': col.toLowerCase() === 'student_id' || col.toLowerCase() === 'total' }"
                  >{{ col }}</span>
                </div>

                <p v-if="missingColumns.length" class="section__error">
                  ⚠ 缺少必需列:{{ missingColumns.join('、') }}
                </p>
                <p v-else class="preview__count">共 {{ preview.dataRowCount }} 行待导入(预览前 {{ preview.rows.length }} 行)</p>

                <div v-if="preview.rows.length" class="preview__table-wrap">
                  <table class="preview__table">
                    <thead>
                      <tr><th v-for="col in preview.columns" :key="col">{{ col }}</th></tr>
                    </thead>
                    <tbody>
                      <tr v-for="(row, ri) in preview.rows" :key="ri">
                        <td v-for="(cell, ci) in preview.columns" :key="ci">{{ row[ci] ?? '' }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>

              <p v-if="importError" class="section__error">{{ importError }}</p>
              <p v-if="importResult" class="section__ok" data-testid="grades-import-result">✓ {{ importResult }}</p>

              <div class="section__actions">
                <button
                  type="button"
                  class="button"
                  :disabled="!canImport"
                  data-testid="grades-import"
                  @click="doImport"
                >{{ importing ? '导入中…' : '导入(不立即发布)' }}</button>
                <button
                  v-if="file"
                  type="button"
                  class="button button--secondary"
                  :disabled="importing"
                  @click="clearFile"
                >移除文件</button>
              </div>
              <p class="section__hint">导入后默认<strong>未发布</strong>,学生看不到;核对无误后再发布。</p>
            </section>

            <!-- PUBLISH -->
            <section v-if="status" class="section">
              <div class="section__label">发布</div>
              <template v-if="status.total === 0">
                <p class="section__muted">导入成绩后可在此发布。</p>
              </template>
              <template v-else-if="status.unpublished === 0">
                <p class="section__ok">✓ 全部 {{ status.published }} 条成绩均已发布。</p>
              </template>
              <template v-else>
                <p class="section__hint">有 <strong>{{ status.unpublished }}</strong> 条待发布。发布后学生立即可见。</p>
                <p v-if="publishError" class="section__error">{{ publishError }}</p>
                <div v-if="!publishConfirm" class="section__actions">
                  <button type="button" class="button" data-testid="grades-publish" @click="publishConfirm = true">
                    发布 {{ status.unpublished }} 条成绩
                  </button>
                </div>
                <div v-else class="confirm">
                  <span class="confirm__text">确认发布?学生将立即看到成绩。</span>
                  <div class="section__actions">
                    <button type="button" class="button button--secondary" :disabled="publishing" @click="publishConfirm = false">取消</button>
                    <button type="button" class="button" :disabled="publishing" data-testid="grades-publish-confirm" @click="doPublish">
                      {{ publishing ? '发布中…' : '确认发布' }}
                    </button>
                  </div>
                </div>
              </template>
            </section>

            <!-- FORMAT HELP -->
            <details class="help">
              <summary class="help__summary">CSV 格式说明</summary>
              <div class="help__body">
                <p>UTF-8、首行表头、<strong>按列名匹配</strong>(列序随意、多余列忽略)。</p>
                <ul class="help__list">
                  <li><code>student_id</code>(必需)— 学号</li>
                  <li><code>total</code>(必需)— 总评</li>
                  <li><code>track</code> / <code>ratio</code> / <code>perf_score</code> / <code>percentile</code> / <code>board_score</code> / <code>remark</code>(选填)</li>
                </ul>
                <p class="help__note">选填数值列留空 = 不显示;某行数值无法解析会导致整批导入失败并提示行号。</p>
                <button type="button" class="button button--secondary" @click="downloadTemplate">下载模板</button>
              </div>
            </details>

            <!-- DANGER -->
            <section v-if="status && status.total > 0" class="section section--danger">
              <div class="section__label">危险操作</div>
              <p v-if="clearError" class="section__error">{{ clearError }}</p>
              <div v-if="!clearConfirm" class="section__actions">
                <button type="button" class="button button--danger" data-testid="grades-clear" @click="clearConfirm = true">
                  清空本 Lab 全部成绩
                </button>
              </div>
              <div v-else class="confirm">
                <span class="confirm__text">将删除全部 {{ status.total }} 条成绩(含已发布),不可撤销。</span>
                <div class="section__actions">
                  <button type="button" class="button button--secondary" :disabled="clearing" @click="clearConfirm = false">取消</button>
                  <button type="button" class="button button--danger" :disabled="clearing" data-testid="grades-clear-confirm" @click="doClear">
                    {{ clearing ? '清空中…' : '确认清空' }}
                  </button>
                </div>
              </div>
            </section>
          </div>

          <div class="drawer__footer">
            <button type="button" class="button button--secondary" :disabled="busy" @click="onClose">关闭</button>
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
  width: min(560px, 92vw);
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
.drawer__close:hover:not(:disabled) { color: var(--text-primary); }

.drawer__body {
  flex: 1;
  overflow-y: auto;
  padding: 18px 20px;
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.drawer__footer {
  padding: 14px 20px;
  border-top: 1px solid var(--border-default);
  display: flex;
  justify-content: flex-end;
  flex-shrink: 0;
}

.section { display: flex; flex-direction: column; gap: 10px; }
.section__label {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 0.68rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}
.section__muted { margin: 0; color: var(--text-tertiary); font-size: 0.85rem; }
.section__hint { margin: 0; color: var(--text-secondary); font-size: 0.8rem; line-height: 1.5; }
.section__error { margin: 0; color: var(--danger); font-size: 0.82rem; }
.section__ok { margin: 0; color: var(--color-open, #2f9e44); font-size: 0.82rem; font-weight: 600; }
.section__actions { display: flex; gap: 8px; flex-wrap: wrap; }
.section--danger { border-top: 1px solid var(--border-default); padding-top: 16px; }

.stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
.stat {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 12px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-default);
  border-radius: 10px;
  text-align: center;
}
.stat__num { font-family: var(--font-mono); font-size: 1.5rem; font-weight: 700; }
.stat__num--ok { color: var(--color-open, #2f9e44); }
.stat__num--pending { color: var(--color-pending, #e8893b); }
.stat__label { color: var(--text-tertiary); font-size: 0.72rem; }

.drop {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  border: 1.5px dashed var(--border-strong);
  border-radius: 10px;
  cursor: pointer;
  background: var(--bg-elevated);
  transition: border-color 120ms ease, background 120ms ease;
}
.drop:hover, .drop--active { border-color: var(--accent); background: color-mix(in srgb, var(--accent) 8%, var(--bg-elevated)); }
.drop__input { display: none; }
.drop__icon { font-size: 1.2rem; color: var(--text-tertiary); }
.drop__text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.drop__text strong { font-size: 0.88rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.drop__sub { color: var(--text-tertiary); font-size: 0.72rem; }

.preview { display: flex; flex-direction: column; gap: 8px; }
.preview__cols { display: flex; flex-wrap: wrap; gap: 6px; }
.chip {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  padding: 2px 8px;
  border-radius: 10px;
  background: var(--bg-hover);
  color: var(--text-secondary);
}
.chip--key { background: color-mix(in srgb, var(--accent) 18%, transparent); color: var(--accent-strong); font-weight: 600; }
.preview__count { margin: 0; color: var(--text-secondary); font-size: 0.8rem; }
.preview__table-wrap { overflow-x: auto; border: 1px solid var(--border-default); border-radius: 8px; }
.preview__table { width: 100%; border-collapse: collapse; font-size: 0.76rem; }
.preview__table th, .preview__table td {
  padding: 6px 10px;
  text-align: left;
  white-space: nowrap;
  border-bottom: 1px solid var(--border-default);
}
.preview__table th { color: var(--text-tertiary); font-family: var(--font-mono); font-weight: 600; background: var(--bg-elevated); }
.preview__table tbody tr:last-child td { border-bottom: none; }

.confirm {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border-default);
  border-radius: 8px;
  background: var(--bg-elevated);
}
.confirm__text { font-size: 0.82rem; color: var(--text-secondary); }

.help { border: 1px solid var(--border-default); border-radius: 8px; background: var(--bg-elevated); }
.help__summary { cursor: pointer; padding: 10px 14px; font-size: 0.82rem; font-weight: 600; }
.help__body { padding: 0 14px 14px; display: flex; flex-direction: column; gap: 8px; font-size: 0.8rem; color: var(--text-secondary); line-height: 1.5; }
.help__list { margin: 0; padding-left: 18px; display: flex; flex-direction: column; gap: 2px; }
.help__list code { font-family: var(--font-mono); font-size: 0.76rem; }
.help__note { margin: 0; color: var(--text-tertiary); font-size: 0.76rem; }

.drawer-enter-active, .drawer-leave-active { transition: opacity 150ms ease; }
.drawer-enter-active .drawer, .drawer-leave-active .drawer { transition: transform 150ms ease; }
.drawer-enter-from, .drawer-leave-to { opacity: 0; }
.drawer-enter-from .drawer, .drawer-leave-to .drawer { transform: translateX(100%); }
</style>
