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

export type TrackTone = 'throughput' | 'latency' | 'fairness';

const TONE_CYCLE: TrackTone[] = ['throughput', 'latency', 'fairness'];

/**
 * Resolve a metric / track id to one of the three canonical track tones.
 * Known CoLab names match by keyword; unknown names fall back to a
 * positional color cycle when `index` is provided, otherwise default
 * to throughput (amber) per the design spec.
 */
export function metricTone(metricId: string, index?: number): TrackTone {
  const value = metricId.toLowerCase();
  if (value.includes('throughput')) return 'throughput';
  if (value.includes('latency')) return 'latency';
  if (value.includes('fair')) return 'fairness';
  if (typeof index === 'number') {
    return TONE_CYCLE[index % TONE_CYCLE.length];
  }
  return 'throughput';
}

export function metricAccentTokens(metricId: string, index?: number) {
  const tone = metricTone(metricId, index);
  return { color: `--track-${tone}`, dim: `--track-${tone}-dim` };
}
