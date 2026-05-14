#!/usr/bin/env bash
# Analyze ranking changes caused by the evaluator image rebuild on 2026-05-14 20:41:23 CST.
#
# Pairing logic (why content_hash is strict):
#   UpsertLeaderboardEntry always overwrites with the latest scored submission.
#   When admin triggered /reeval, it took each user's current leaderboard entry
#   (= their most recent pre-cutoff scored submission) and re-submitted the same
#   artifact. Post-cutoff submissions with a matching content_hash ARE the reeval.
#
#   pre-reeval score  = most recent pre-cutoff scored submission's score (old evaluator)
#   post-reeval score = earliest post-cutoff scored submission with same content_hash
#                       (new evaluator), divided by 1000 to normalize scale
#
# Outputs:
#   1. Summary: submission counts, coverage
#   2. Per-user rank change table (combined ranking, each user sorted by their
#      selected track, matching the actual leaderboard behaviour)
#   3. CSV file: pre-reeval ranking exported to ./pre-reeval-ranking.csv
#
# Usage (run from repo root on the production machine):
#   bash scripts/analyze-ranking-inversion.sh [container-name]

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$REPO_ROOT/deploy/.env.prod"
CONTAINER="${1:-deploy-postgres-1}"
LAB_ID="colab-2026-p2"
# Exact image rebuild timestamp: 2026-05-14T12:47:42.828431801Z
CUTOFF="2026-05-14 12:47:42.828431801+00"
CSV_OUT="$REPO_ROOT/pre-reeval-ranking.csv"

if [[ ! -f "$ENV_FILE" ]]; then
    echo "ERROR: $ENV_FILE not found." >&2
    exit 1
fi

parse_env() {
    grep "^${1}=" "$ENV_FILE" | head -1 | sed "s/^${1}=//; s/['\"]//g"
}

PG_USER=$(parse_env POSTGRES_USER)
PG_DB=$(parse_env POSTGRES_DB)
PG_PASS=$(parse_env POSTGRES_PASSWORD)

run_sql() {
    docker exec -i -e PGPASSWORD="$PG_PASS" "$CONTAINER" \
        psql -U "$PG_USER" -d "$PG_DB" --no-psqlrc "$@"
}

echo "========================================================"
echo "  Ranking Change Analysis — $LAB_ID"
echo "  Cutoff : 2026-05-14T12:47:42Z (image rebuild time)"
echo "  Container: $CONTAINER"
echo "========================================================"

# ── 1. Overview ──────────────────────────────────────────────────────────────
echo ""
echo "── 1. Scored submission counts ──"
run_sql -c "
SELECT
    CASE WHEN created_at >= TIMESTAMPTZ '$CUTOFF'
         THEN 'after cutoff (new eval)' ELSE 'before cutoff (old eval)' END AS grp,
    COUNT(*)                AS submissions,
    COUNT(DISTINCT user_id) AS distinct_users
FROM submissions
WHERE lab_id = '$LAB_ID' AND verdict = 'scored'
GROUP BY grp ORDER BY grp;
"

echo ""
echo "── 2. Reeval pair coverage ──"
run_sql -c "
WITH
pre AS (
    -- Most recent pre-cutoff scored submission per user
    SELECT DISTINCT ON (user_id) user_id, content_hash
    FROM submissions
    WHERE lab_id = '$LAB_ID' AND verdict = 'scored'
      AND created_at < TIMESTAMPTZ '$CUTOFF'
    ORDER BY user_id, created_at DESC
),
post AS (
    SELECT DISTINCT user_id, content_hash
    FROM submissions
    WHERE lab_id = '$LAB_ID' AND verdict = 'scored'
      AND created_at >= TIMESTAMPTZ '$CUTOFF'
)
SELECT
    COUNT(DISTINCT pre.user_id)                       AS total_pre_users,
    COUNT(DISTINCT post.user_id)                      AS total_post_users,
    COUNT(DISTINCT pre.user_id)
      FILTER (WHERE post.user_id IS NOT NULL)         AS paired_users,
    COUNT(DISTINCT pre.user_id)
      FILTER (WHERE post.user_id IS NULL)             AS unpaired_pre_only
FROM pre LEFT JOIN post ON pre.user_id = post.user_id
                       AND pre.content_hash = post.content_hash;
"

# ── 3. Rank change table ──────────────────────────────────────────────────────
echo ""
echo "── 3. Per-user rank change (combined ranking, sorted by own track) ──"

run_sql <<EOSQL
\pset format aligned
\pset border 1

WITH
cutoff AS (SELECT TIMESTAMPTZ '$CUTOFF' AS ts),

-- Most recent pre-cutoff scored submission per user
pre_latest AS (
    SELECT DISTINCT ON (user_id)
        id AS sub_id, user_id, content_hash
    FROM submissions
    WHERE lab_id = '$LAB_ID' AND verdict = 'scored'
      AND created_at < (SELECT ts FROM cutoff)
    ORDER BY user_id, created_at DESC
),

-- Earliest post-cutoff scored submission with same content_hash = reeval
post_reeval AS (
    SELECT DISTINCT ON (pl.user_id)
        s.id AS sub_id, pl.user_id
    FROM pre_latest pl
    JOIN submissions s ON s.user_id = pl.user_id
                      AND s.content_hash = pl.content_hash
                      AND s.lab_id = '$LAB_ID'
                      AND s.verdict = 'scored'
                      AND s.created_at >= (SELECT ts FROM cutoff)
    ORDER BY pl.user_id, s.created_at ASC
),

-- User profile: selected track (default throughput) and nickname
profiles AS (
    SELECT user_id,
        COALESCE(NULLIF(TRIM(track), ''), 'throughput') AS track,
        COALESCE(NULLIF(TRIM(nickname), ''), '匿名')    AS nickname
    FROM lab_profiles
    WHERE lab_id = '$LAB_ID'
),

-- Pre-reeval: all metric scores, compute sort key by selected track
pre_scores AS (
    SELECT
        pl.user_id,
        COALESCE(p.track, 'throughput')    AS track,
        COALESCE(p.nickname, '匿名')       AS nickname,
        MAX(CASE WHEN sc.metric_id = 'throughput' THEN sc.value::float8 END) AS t,
        MAX(CASE WHEN sc.metric_id = 'latency'    THEN sc.value::float8 END) AS l,
        MAX(CASE WHEN sc.metric_id = 'fairness'   THEN sc.value::float8 END) AS f
    FROM pre_latest pl
    LEFT JOIN profiles p ON p.user_id = pl.user_id
    JOIN scores sc ON sc.submission_id = pl.sub_id
    GROUP BY pl.user_id, p.track, p.nickname
),
pre_with_sort AS (
    SELECT *,
        CASE track
            WHEN 'latency'  THEN l
            WHEN 'fairness' THEN f
            ELSE t
        END AS sort_val
    FROM pre_scores
),
pre_ranked AS (
    SELECT user_id, track, nickname, t, l, f, sort_val,
        ROW_NUMBER() OVER (ORDER BY sort_val DESC NULLS LAST) AS old_rank
    FROM pre_with_sort
),

-- Post-reeval: metric scores divided by 1000, compute sort key
post_scores AS (
    SELECT
        pr.user_id,
        MAX(CASE WHEN sc.metric_id = 'throughput' THEN sc.value::float8 / 1000.0 END) AS t,
        MAX(CASE WHEN sc.metric_id = 'latency'    THEN sc.value::float8 / 1000.0 END) AS l,
        MAX(CASE WHEN sc.metric_id = 'fairness'   THEN sc.value::float8 / 1000.0 END) AS f
    FROM post_reeval pr
    JOIN scores sc ON sc.submission_id = pr.sub_id
    GROUP BY pr.user_id
),
post_with_sort AS (
    SELECT ps.*,
        CASE COALESCE(p.track, 'throughput')
            WHEN 'latency'  THEN ps.l
            WHEN 'fairness' THEN ps.f
            ELSE ps.t
        END AS sort_val
    FROM post_scores ps
    LEFT JOIN profiles p ON p.user_id = ps.user_id
),
post_ranked AS (
    SELECT user_id, sort_val,
        ROW_NUMBER() OVER (ORDER BY sort_val DESC NULLS LAST) AS new_rank
    FROM post_with_sort
)

-- Summary counts
SELECT
    COUNT(*)                                          AS total_paired,
    COUNT(*) FILTER (WHERE pr.old_rank <> nr.new_rank) AS rank_changed,
    COUNT(*) FILTER (WHERE nr.new_rank < pr.old_rank)  AS rank_improved,
    COUNT(*) FILTER (WHERE nr.new_rank > pr.old_rank)  AS rank_declined
FROM pre_ranked pr
JOIN post_ranked nr ON nr.user_id = pr.user_id;

\echo ''
\echo 'All paired users — rank before and after reeval (sorted by |rank_delta| desc):'

WITH
cutoff AS (SELECT TIMESTAMPTZ '$CUTOFF' AS ts),
pre_latest AS (
    SELECT DISTINCT ON (user_id) id AS sub_id, user_id, content_hash
    FROM submissions
    WHERE lab_id = '$LAB_ID' AND verdict = 'scored'
      AND created_at < (SELECT ts FROM cutoff)
    ORDER BY user_id, created_at DESC
),
post_reeval AS (
    SELECT DISTINCT ON (pl.user_id) s.id AS sub_id, pl.user_id
    FROM pre_latest pl
    JOIN submissions s ON s.user_id = pl.user_id
                      AND s.content_hash = pl.content_hash
                      AND s.lab_id = '$LAB_ID' AND s.verdict = 'scored'
                      AND s.created_at >= (SELECT ts FROM cutoff)
    ORDER BY pl.user_id, s.created_at ASC
),
profiles AS (
    SELECT user_id,
        COALESCE(NULLIF(TRIM(track), ''), 'throughput') AS track,
        COALESCE(NULLIF(TRIM(nickname), ''), '匿名')    AS nickname
    FROM lab_profiles WHERE lab_id = '$LAB_ID'
),
pre_scores AS (
    SELECT pl.user_id,
        COALESCE(p.track, 'throughput') AS track,
        COALESCE(p.nickname, '匿名')    AS nickname,
        MAX(CASE WHEN sc.metric_id = 'throughput' THEN sc.value::float8 END) AS t,
        MAX(CASE WHEN sc.metric_id = 'latency'    THEN sc.value::float8 END) AS l,
        MAX(CASE WHEN sc.metric_id = 'fairness'   THEN sc.value::float8 END) AS f
    FROM pre_latest pl
    LEFT JOIN profiles p ON p.user_id = pl.user_id
    JOIN scores sc ON sc.submission_id = pl.sub_id
    GROUP BY pl.user_id, p.track, p.nickname
),
pre_with_sort AS (
    SELECT *, CASE track WHEN 'latency' THEN l WHEN 'fairness' THEN f ELSE t END AS sort_val
    FROM pre_scores
),
pre_ranked AS (
    SELECT user_id, track, nickname, t, l, f, sort_val,
        ROW_NUMBER() OVER (ORDER BY sort_val DESC NULLS LAST) AS old_rank
    FROM pre_with_sort
),
post_scores AS (
    SELECT pr.user_id,
        MAX(CASE WHEN sc.metric_id = 'throughput' THEN sc.value::float8/1000 END) AS t,
        MAX(CASE WHEN sc.metric_id = 'latency'    THEN sc.value::float8/1000 END) AS l,
        MAX(CASE WHEN sc.metric_id = 'fairness'   THEN sc.value::float8/1000 END) AS f
    FROM post_reeval pr
    JOIN scores sc ON sc.submission_id = pr.sub_id
    GROUP BY pr.user_id
),
post_with_sort AS (
    SELECT ps.*, CASE COALESCE(p.track,'throughput') WHEN 'latency' THEN ps.l WHEN 'fairness' THEN ps.f ELSE ps.t END AS sort_val
    FROM post_scores ps LEFT JOIN profiles p ON p.user_id = ps.user_id
),
post_ranked AS (
    SELECT user_id, ROW_NUMBER() OVER (ORDER BY sort_val DESC NULLS LAST) AS new_rank
    FROM post_with_sort
)
SELECT
    u.student_id,
    pr.track,
    ROUND(pr.sort_val::numeric, 4)          AS old_track_score,
    ROUND(nr_ps.sort_val::numeric, 4)       AS new_track_score,
    pr.old_rank,
    nr.new_rank,
    (nr.new_rank::int - pr.old_rank::int)   AS rank_delta
FROM pre_ranked pr
JOIN post_ranked nr ON nr.user_id = pr.user_id
JOIN post_with_sort nr_ps ON nr_ps.user_id = pr.user_id
JOIN users u ON u.id = pr.user_id
ORDER BY ABS(nr.new_rank::int - pr.old_rank::int) DESC, pr.old_rank;
EOSQL

# ── 4. Export pre-reeval ranking as CSV ──────────────────────────────────────
echo ""
echo "── 4. Exporting pre-reeval ranking to $CSV_OUT ──"

run_sql --csv <<EOSQL > "$CSV_OUT"
WITH
cutoff AS (SELECT TIMESTAMPTZ '$CUTOFF' AS ts),

profiles AS (
    SELECT user_id,
        COALESCE(NULLIF(TRIM(track), ''), 'throughput') AS track,
        COALESCE(NULLIF(TRIM(nickname), ''), '匿名')    AS nickname
    FROM lab_profiles WHERE lab_id = '$LAB_ID'
),

-- All scored submissions in the 1-hour window before cutoff, with all metric scores
window_scores AS (
    SELECT
        s.id AS sub_id,
        s.user_id,
        COALESCE(p.track, 'throughput') AS track,
        COALESCE(p.nickname, '匿名')    AS nickname,
        MAX(CASE WHEN sc.metric_id = 'throughput' THEN sc.value::float8 END) AS throughput,
        MAX(CASE WHEN sc.metric_id = 'latency'    THEN sc.value::float8 END) AS latency,
        MAX(CASE WHEN sc.metric_id = 'fairness'   THEN sc.value::float8 END) AS fairness
    FROM submissions s
    LEFT JOIN profiles p ON p.user_id = s.user_id
    JOIN scores sc ON sc.submission_id = s.id
    WHERE s.lab_id = '$LAB_ID'
      AND s.verdict = 'scored'
      AND s.created_at >= (SELECT ts - INTERVAL '1 hour' FROM cutoff)
      AND s.created_at <  (SELECT ts FROM cutoff)
    GROUP BY s.id, s.user_id, p.track, p.nickname
),

-- Compute sort key (score on user's selected track) per submission
window_with_sort AS (
    SELECT *,
        CASE track
            WHEN 'latency'  THEN latency
            WHEN 'fairness' THEN fairness
            ELSE throughput
        END AS sort_val
    FROM window_scores
),

-- Best submission per user in the window (highest score on their track)
best_in_window AS (
    SELECT DISTINCT ON (user_id)
        user_id, track, nickname, throughput, latency, fairness, sort_val
    FROM window_with_sort
    ORDER BY user_id, sort_val DESC NULLS LAST
),

ranked AS (
    SELECT
        ROW_NUMBER() OVER (ORDER BY sort_val DESC NULLS LAST) AS rank,
        user_id, track, nickname, throughput, latency, fairness, sort_val
    FROM best_in_window
)
SELECT
    rank,
    u.student_id,
    r.nickname,
    r.track,
    ROUND(r.throughput::numeric, 4) AS throughput,
    ROUND(r.latency::numeric,    4) AS latency,
    ROUND(r.fairness::numeric,   4) AS fairness,
    ROUND(r.sort_val::numeric,   4) AS track_score
FROM ranked r
JOIN users u ON u.id = r.user_id
ORDER BY rank;
EOSQL

echo "CSV written to: $CSV_OUT"
echo ""
echo "========================================================"
echo "  Done."
echo "========================================================"
