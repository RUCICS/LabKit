import { readAPIError } from './http';

export type GradeItem = {
  label: string;
  value: string;
};

export type FinalGrade = {
  lab_id: string;
  student_id: string;
  total?: string;
  items: GradeItem[];
  remark?: string;
  published_at?: string;
  updated_at: string;
};

export type GradeResult =
  | { status: 'ok'; grade: FinalGrade }
  | { status: 'unauthorized' }
  | { status: 'unpublished' }
  | { status: 'error'; message: string };

export type GradesResult =
  | { status: 'ok'; grades: FinalGrade[] }
  | { status: 'unauthorized' }
  | { status: 'error'; message: string };

// getMyGrades lists every published grade for the signed-in student across all
// labs — the /grade landing assumes no particular lab.
export async function getMyGrades(): Promise<GradesResult> {
  try {
    const response = await fetch('/api/grades', { credentials: 'include' });
    if (response.status === 401) {
      return { status: 'unauthorized' };
    }
    if (!response.ok) {
      return { status: 'error', message: await readAPIError(response, '加载成绩失败') };
    }
    const payload = (await response.json()) as { grades?: FinalGrade[] };
    return { status: 'ok', grades: payload.grades ?? [] };
  } catch (error) {
    return {
      status: 'error',
      message: error instanceof Error ? error.message : '加载成绩失败'
    };
  }
}

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
