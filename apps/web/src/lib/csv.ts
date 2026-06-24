// Minimal RFC 4180-ish CSV parser for client-side preview. Handles quoted
// fields, escaped quotes (""), and commas/newlines inside quotes. The server
// remains the source of truth; this only powers the upload preview.
export function parseCsv(input: string): string[][] {
  let text = input;
  if (text.charCodeAt(0) === 0xfeff) {
    text = text.slice(1); // strip BOM
  }

  const rows: string[][] = [];
  let row: string[] = [];
  let field = '';
  let inQuotes = false;
  let i = 0;

  while (i < text.length) {
    const ch = text[i];
    if (inQuotes) {
      if (ch === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i += 2;
          continue;
        }
        inQuotes = false;
        i += 1;
        continue;
      }
      field += ch;
      i += 1;
      continue;
    }
    if (ch === '"') {
      inQuotes = true;
      i += 1;
      continue;
    }
    if (ch === ',') {
      row.push(field);
      field = '';
      i += 1;
      continue;
    }
    if (ch === '\r') {
      i += 1;
      continue;
    }
    if (ch === '\n') {
      row.push(field);
      rows.push(row);
      row = [];
      field = '';
      i += 1;
      continue;
    }
    field += ch;
    i += 1;
  }
  if (field !== '' || row.length > 0) {
    row.push(field);
    rows.push(row);
  }
  return rows;
}

export type GradeCsvPreview = {
  columns: string[];
  rows: string[][];
  dataRowCount: number;
  hasStudentId: boolean;
};

// The only column the importer requires (mirrors apps/api grade service); every
// other column is optional and shown to students as a free-form breakdown.
export const REQUIRED_GRADE_COLUMNS = ['student_id'] as const;

export function previewGradeCsv(text: string, maxRows = 5): GradeCsvPreview {
  const all = parseCsv(text).filter((cells) => cells.some((cell) => cell.trim() !== ''));
  const header = all[0] ?? [];
  const columns = header.map((cell) => cell.trim());
  const lower = columns.map((cell) => cell.toLowerCase());
  const dataRows = all.slice(1);
  return {
    columns,
    rows: dataRows.slice(0, maxRows),
    dataRowCount: dataRows.length,
    hasStudentId: lower.includes('student_id')
  };
}
