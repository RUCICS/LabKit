export type LeaderboardMetricSort = 'asc' | 'desc';

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
  scores: LeaderboardScore[];
  updated_at: string;
}

export interface LeaderboardBoard {
  lab_id: string;
  selected_metric: string;
  metrics: LeaderboardMetric[];
  rows: LeaderboardRow[];
}

export interface LeaderboardLabDetail {
  id: string;
  name: string;
  manifest?: {
    schedule?: {
      visible?: string;
      open?: string;
      close?: string;
    };
    metrics?: Array<{
      id: string;
      name: string;
      sort: LeaderboardMetricSort;
      unit?: string;
    }>;
  };
}

export interface PublicLab {
  id: string;
  name: string;
  manifest?: {
    submit?: {
      files?: string[];
    };
    metrics?: Array<{
      id: string;
      name: string;
      sort: LeaderboardMetricSort;
      unit?: string;
    }>;
    schedule?: {
      visible?: string;
      open?: string;
      close?: string;
    };
  };
}
