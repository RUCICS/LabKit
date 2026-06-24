-- name: UpsertFinalGrade :one
INSERT INTO final_grades (
    lab_id, student_id, total, remark, items, updated_at
)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (lab_id, student_id) DO UPDATE SET
    total = EXCLUDED.total,
    remark = EXCLUDED.remark,
    items = EXCLUDED.items,
    updated_at = NOW()
RETURNING *;

-- name: GetFinalGrade :one
SELECT *
FROM final_grades
WHERE lab_id = $1
  AND student_id = $2
  AND published_at IS NOT NULL
  AND published_at <= NOW()
LIMIT 1;

-- name: PublishFinalGrades :execrows
UPDATE final_grades
SET published_at = NOW(),
    updated_at = NOW()
WHERE lab_id = $1
  AND published_at IS NULL;

-- name: DeleteFinalGradesByLab :execrows
DELETE FROM final_grades
WHERE lab_id = $1;

-- name: SummarizeFinalGrades :one
SELECT
    COUNT(*)                   AS total,
    COUNT(published_at)        AS published,
    MAX(updated_at)::timestamptz AS last_updated
FROM final_grades
WHERE lab_id = $1;
