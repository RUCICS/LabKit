import { describe, expect, it } from 'vitest';
import { parseCsv, previewGradeCsv } from './csv';

describe('parseCsv', () => {
  it('parses simple rows', () => {
    expect(parseCsv('a,b,c\n1,2,3\n')).toEqual([
      ['a', 'b', 'c'],
      ['1', '2', '3']
    ]);
  });

  it('handles quoted fields with commas and escaped quotes', () => {
    const rows = parseCsv('student_id,remark\n2026001,"hello, world"\n2026002,"say ""hi"""\n');
    expect(rows[1]).toEqual(['2026001', 'hello, world']);
    expect(rows[2]).toEqual(['2026002', 'say "hi"']);
  });

  it('strips a UTF-8 BOM', () => {
    const rows = parseCsv('﻿student_id,total\n2026001,90\n');
    expect(rows[0]).toEqual(['student_id', 'total']);
  });

  it('tolerates a missing trailing newline', () => {
    expect(parseCsv('a,b\n1,2')).toEqual([
      ['a', 'b'],
      ['1', '2']
    ]);
  });
});

describe('previewGradeCsv', () => {
  it('reports columns, row count and student_id presence', () => {
    const preview = previewGradeCsv(
      'student_id,track,total,remark\n2026001,t,90,\n2026002,t,80,ok\n'
    );
    expect(preview.columns).toEqual(['student_id', 'track', 'total', 'remark']);
    expect(preview.dataRowCount).toBe(2);
    expect(preview.hasStudentId).toBe(true);
  });

  it('detects student_id case-insensitively', () => {
    expect(previewGradeCsv('Student_ID,Track\n2026001,t\n').hasStudentId).toBe(true);
    expect(previewGradeCsv('name,total\nAda,90\n').hasStudentId).toBe(false);
  });

  it('ignores blank lines when counting data rows', () => {
    const preview = previewGradeCsv('student_id,total\n2026001,90\n\n2026002,80\n');
    expect(preview.dataRowCount).toBe(2);
  });
});
