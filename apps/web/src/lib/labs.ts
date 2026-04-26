import type { LabManifestMetric, LabManifestSchedule, QuotaSummary } from '../components/board/types';

export type LabPhase = 'upcoming' | 'open' | 'closed';

function scheduleValue(schedule: LabManifestSchedule | undefined, key: 'visible' | 'open' | 'close') {
  if (!schedule) return '';
  const direct = schedule[key];
  if (typeof direct === 'string' && direct.trim()) {
    return direct;
  }
  const capitalized = (key.charAt(0).toUpperCase() + key.slice(1)) as 'Visible' | 'Open' | 'Close';
  const fallback = schedule[capitalized];
  return typeof fallback === 'string' ? fallback : '';
}

export function getLabSchedule(manifest: { schedule?: LabManifestSchedule; Schedule?: LabManifestSchedule } | undefined) {
  return manifest?.schedule ?? manifest?.Schedule;
}

export function getLabPhase(schedule?: LabManifestSchedule): LabPhase {
  const now = Date.now();
  const open = Date.parse(scheduleValue(schedule, 'open'));
  const close = Date.parse(scheduleValue(schedule, 'close'));
  if (!Number.isNaN(open) && now < open) {
    return 'upcoming';
  }
  if (!Number.isNaN(close) && now > close) {
    return 'closed';
  }
  return 'open';
}

export function getLabCloseISO(schedule?: LabManifestSchedule) {
  return scheduleValue(schedule, 'close');
}

export function getLabMetrics(manifest: { metrics?: LabManifestMetric[]; Metrics?: LabManifestMetric[] } | undefined) {
  return manifest?.metrics ?? manifest?.Metrics ?? [];
}

export function labPhaseLabel(phase: LabPhase) {
  switch (phase) {
    case 'upcoming':
      return 'UPCOMING';
    case 'closed':
      return 'CLOSED';
    default:
      return 'OPEN';
  }
}

export function formatQuotaSummary(quota?: QuotaSummary | null) {
  if (!quota) {
    return '';
  }
  return `${quota.left} left today · ${quota.used}/${quota.daily} used`;
}

export type TrackTone = 'amber' | 'cyan' | 'purple';

const TONES: TrackTone[] = ['amber', 'cyan', 'purple'];

export function metricTone(index: number): TrackTone {
  return TONES[index % TONES.length];
}

export function metricAccentTokens(index: number) {
  const tone = metricTone(index);
  return { color: `--tone-${tone}`, dim: `--tone-${tone}-dim` };
}
