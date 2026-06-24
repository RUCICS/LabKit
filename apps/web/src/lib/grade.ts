import { readAPIError } from './http';

// Default lab shown at the bare /grade route. Lab2 (CoLab) is the first lab to
// publish a final course grade through LabKit.
export const DEFAULT_GRADE_LAB_ID = 'colab-2026-p2';

export type FinalGrade = {
  lab_id: string;
  student_id: string;
  total: number;
  track?: string;
  ratio?: number;
  perf_score?: number;
  percentile?: number;
  board_score?: number;
  remark?: string;
  published_at?: string;
  updated_at: string;
};

export type GradeResult =
  | { status: 'ok'; grade: FinalGrade }
  | { status: 'unauthorized' }
  | { status: 'unpublished' }
  | { status: 'error'; message: string };

export async function getMyGrade(labId: string): Promise<GradeResult> {
  try {
    const response = await fetch(`/api/labs/${encodeURIComponent(labId)}/grade`, {
      credentials: 'include'
    });
    if (response.status === 401) {
      return { status: 'unauthorized' };
    }
    if (response.status === 404) {
      return { status: 'unpublished' };
    }
    if (!response.ok) {
      return { status: 'error', message: await readAPIError(response, '加载成绩失败') };
    }
    const grade = (await response.json()) as FinalGrade;
    return { status: 'ok', grade };
  } catch (error) {
    return {
      status: 'error',
      message: error instanceof Error ? error.message : '加载成绩失败'
    };
  }
}
