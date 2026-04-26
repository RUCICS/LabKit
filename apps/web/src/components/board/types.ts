export type LeaderboardMetricSort = 'asc' | 'desc';

export interface QuotaSummary {
  daily: number;
  used: number;
  left: number;
  reset_hint: string;
}

export interface LeaderboardMetric {
  id: string;
  name: string;
  sort: LeaderboardMetricSort;
  selected?: boolean;
}

export interface LeaderboardScore {
  metric_id: string;
  value: number;
}

export interface LeaderboardRow {
  rank: number;
  nickname: string;
  track?: string;
  current_user?: boolean;
  scores: LeaderboardScore[];
  updated_at: string;
}

export interface LeaderboardBoard {
  lab_id: string;
  selected_metric: string;
  metrics: LeaderboardMetric[];
  rows: LeaderboardRow[];
  quota?: QuotaSummary;
}

export interface LabManifestMetric {
  id: string;
  name: string;
  sort: LeaderboardMetricSort;
  unit?: string;
}

export interface LabManifestSchedule {
  visible?: string;
  open?: string;
  close?: string;
  // API payloads coming from Go structs may use exported field names.
  Visible?: string;
  Open?: string;
  Close?: string;
}

export interface LabManifestBoard {
  pick?: boolean;
  rank_by?: string;
  Pick?: boolean;
  RankBy?: string;
}

export interface LabManifest {
  lab?: {
    tags?: Record<string, string>;
  };
  board?: LabManifestBoard;
  schedule?: LabManifestSchedule;
  metrics?: LabManifestMetric[];
  // API payloads coming from Go structs may use exported field names.
  Lab?: {
    Tags?: Record<string, string>;
  };
  Board?: LabManifestBoard;
  Schedule?: LabManifestSchedule;
  Metrics?: LabManifestMetric[];
}

export interface LeaderboardLabDetail {
  id: string;
  name: string;
  manifest?: LabManifest;
}

export interface PublicLab {
  id: string;
  name: string;
  manifest?: LabManifest & {
    submit?: {
      files?: string[];
    };
  };
}
