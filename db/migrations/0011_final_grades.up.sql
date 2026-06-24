BEGIN;

CREATE TABLE final_grades (
    lab_id       TEXT        NOT NULL,
    student_id   TEXT        NOT NULL,
    total        REAL        NOT NULL,            -- 总评
    track        TEXT,                            -- 选定赛道
    ratio        REAL,                            -- r(赛道倍率)
    perf_score   REAL,                            -- 性能分(85%)
    percentile   REAL,                            -- p(赛道内百分位)
    board_score  REAL,                            -- 打榜分(15%)
    remark       TEXT,                            -- 申诉 / 备注
    published_at TIMESTAMPTZ,                     -- NULL=暂存不可见;有值=学生可见
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (lab_id, student_id)
);

COMMIT;
