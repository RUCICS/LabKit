-- name: UpsertFinalGrade :one
INSERT INTO final_grades (
    lab_id, student_id, total, track, ratio, perf_score, percentile, board_score, remark, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
ON CONFLICT (lab_id, student_id) DO UPDATE SET
    total = EXCLUDED.total,
    track = EXCLUDED.track,
    ratio = EXCLUDED.ratio,
    perf_score = EXCLUDED.perf_score,
    percentile = EXCLUDED.percentile,
    board_score = EXCLUDED.board_score,
    remark = EXCLUDED.remark,
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
