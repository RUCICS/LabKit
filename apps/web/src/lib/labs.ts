import type { LabManifestSchedule, QuotaSummary } from '../components/board/types';

export type LabPhase = 'upcoming' | 'open' | 'closed';

export function getLabPhase(schedule?: LabManifestSchedule): LabPhase {
  const now = Date.now();
  const open = Date.parse(schedule?.open ?? '');
  const close = Date.parse(schedule?.close ?? '');
  if (!Number.isNaN(open) && now < open) {
    return 'upcoming';
  }
  if (!Number.isNaN(close) && now > close) {
    return 'closed';
  }
  return 'open';
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
