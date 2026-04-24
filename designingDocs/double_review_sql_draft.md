# 双评分员完整 SQL 草案

本草案面向当前 `SkillJudge` 平台库（PostgreSQL），目标是：

1. 在现有单评结构上扩展出双评分员能力。
2. 保留旧数据兼容，不立即删除 `videos.scorer_id`。
3. 提供可直接运行的 SQL，包括：
   - 建表 / 改表迁移
   - 单评历史数据回填
   - 双评分配事务模板
   - 提交人工评分并重算汇总的事务模板

注意：

- 文中的运行期 SQL 模板使用了真实 SQL 结构，但需要你把其中的示例 UUID、分数、JSON 替换成真实参数后再执行。
- 本文只解决数据库与 SQL 层设计，不代表当前 Go 代码已经切到双评模型。

## 1. 目标结构

双评第一阶段按以下思路落地：

- `video_review_assignments`
  - 一条记录表示“某个视频分配给某位评分员的一次评审槽位”
- `manual_evaluations`
  - 增加 `assignment_id`
  - 一条人工评分明细对应一条 assignment
- `videos`
  - 保留为视频主表 + 最终人工汇总表
  - `manual_score` 表示最终人工分
  - `manual_status` 表示整条视频的人工作业汇总状态

## 2. 迁移 SQL

这部分是一次性迁移脚本。

```sql
BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

------------------------------------------------------------
-- 2.1 tasks 增加 review 配置字段
------------------------------------------------------------
ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS required_review_count SMALLINT NOT NULL DEFAULT 1;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_tasks_required_review_count'
    ) THEN
        ALTER TABLE tasks
            ADD CONSTRAINT chk_tasks_required_review_count
            CHECK (required_review_count >= 1);
    END IF;
END $$;

------------------------------------------------------------
-- 2.2 新表：video_review_assignments
------------------------------------------------------------
CREATE TABLE IF NOT EXISTS video_review_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    video_id UUID NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    scorer_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    review_no SMALLINT NOT NULL,
    review_type VARCHAR(20) NOT NULL DEFAULT 'normal',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',

    assigned_at TIMESTAMP NULL,
    started_at TIMESTAMP NULL,
    submitted_at TIMESTAMP NULL,

    source_assignment_id UUID NULL REFERENCES video_review_assignments(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_vra_review_no_positive'
    ) THEN
        ALTER TABLE video_review_assignments
            ADD CONSTRAINT chk_vra_review_no_positive
            CHECK (review_no > 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_vra_review_type'
    ) THEN
        ALTER TABLE video_review_assignments
            ADD CONSTRAINT chk_vra_review_type
            CHECK (review_type IN ('normal', 'arbitration', 'rescore'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_vra_status'
    ) THEN
        ALTER TABLE video_review_assignments
            ADD CONSTRAINT chk_vra_status
            CHECK (status IN ('pending', 'in_progress', 'submitted', 'cancelled'));
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_vra_video_review_no
    ON video_review_assignments(video_id, review_no);

CREATE UNIQUE INDEX IF NOT EXISTS uq_vra_video_scorer_review_type
    ON video_review_assignments(video_id, scorer_id, review_type);

CREATE INDEX IF NOT EXISTS idx_vra_task_video
    ON video_review_assignments(task_id, video_id);

CREATE INDEX IF NOT EXISTS idx_vra_scorer_status_assigned_at
    ON video_review_assignments(scorer_id, status, assigned_at);

CREATE INDEX IF NOT EXISTS idx_vra_video_status
    ON video_review_assignments(video_id, status);

------------------------------------------------------------
-- 2.3 manual_evaluations 增加 assignment_id
------------------------------------------------------------
ALTER TABLE manual_evaluations
    ADD COLUMN IF NOT EXISTS assignment_id UUID NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_manual_evaluations_assignment'
    ) THEN
        ALTER TABLE manual_evaluations
            ADD CONSTRAINT fk_manual_evaluations_assignment
            FOREIGN KEY (assignment_id)
            REFERENCES video_review_assignments(id)
            ON DELETE RESTRICT;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'uq_manual_evaluations_assignment_id'
    ) THEN
        ALTER TABLE manual_evaluations
            ADD CONSTRAINT uq_manual_evaluations_assignment_id
            UNIQUE (assignment_id);
    END IF;
END $$;

------------------------------------------------------------
-- 2.4 videos 增加双评汇总字段
------------------------------------------------------------
ALTER TABLE videos
    ADD COLUMN IF NOT EXISTS required_review_count SMALLINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS submitted_review_count SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS score_decision_type VARCHAR(20) NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_videos_required_review_count'
    ) THEN
        ALTER TABLE videos
            ADD CONSTRAINT chk_videos_required_review_count
            CHECK (required_review_count >= 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_videos_submitted_review_count'
    ) THEN
        ALTER TABLE videos
            ADD CONSTRAINT chk_videos_submitted_review_count
            CHECK (submitted_review_count >= 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_videos_score_decision_type'
    ) THEN
        ALTER TABLE videos
            ADD CONSTRAINT chk_videos_score_decision_type
            CHECK (
                score_decision_type IS NULL
                OR score_decision_type IN ('single', 'average', 'arbitration', 'manual_select')
            );
    END IF;
END $$;

------------------------------------------------------------
-- 2.5 汇总函数：每次提交人工评分后调用
------------------------------------------------------------
CREATE OR REPLACE FUNCTION refresh_video_manual_summary(p_video_id UUID)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    v_required_review_count SMALLINT;
    v_ai_status VARCHAR(20);
    v_submitted_count INTEGER;
    v_avg_score NUMERIC(5,2);
    v_manual_status VARCHAR(20);
    v_score_decision_type VARCHAR(20);
    v_evaluation_status VARCHAR(20);
    v_completed_at TIMESTAMP;
BEGIN
    SELECT
        COALESCE(required_review_count, 1),
        COALESCE(ai_status, 'pending')
    INTO
        v_required_review_count,
        v_ai_status
    FROM videos
    WHERE id = p_video_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'video % not found', p_video_id;
    END IF;

    SELECT
        COUNT(*)::INTEGER,
        ROUND(AVG(me.total_score)::numeric, 2)
    INTO
        v_submitted_count,
        v_avg_score
    FROM video_review_assignments vra
    JOIN manual_evaluations me
      ON me.assignment_id = vra.id
    WHERE vra.video_id = p_video_id
      AND vra.review_type = 'normal'
      AND vra.status = 'submitted';

    IF v_submitted_count <= 0 THEN
        v_manual_status := 'pending';
        v_avg_score := NULL;
        v_score_decision_type := NULL;
    ELSIF v_submitted_count < v_required_review_count THEN
        v_manual_status := 'in_progress';
        v_avg_score := NULL;
        v_score_decision_type := NULL;
    ELSE
        v_manual_status := 'submitted';
        v_score_decision_type := CASE
            WHEN v_required_review_count = 1 THEN 'single'
            ELSE 'average'
        END;
    END IF;

    v_evaluation_status := CASE
        WHEN v_ai_status = 'failed' THEN 'failed'
        WHEN v_manual_status = 'submitted' AND v_ai_status = 'completed' THEN 'completed'
        WHEN v_manual_status = 'pending' AND v_ai_status = 'pending' THEN 'pending'
        ELSE 'in_progress'
    END;

    v_completed_at := CASE
        WHEN v_evaluation_status = 'completed' THEN CURRENT_TIMESTAMP
        ELSE NULL
    END;

    UPDATE videos
    SET submitted_review_count = v_submitted_count,
        manual_status = v_manual_status,
        manual_score = CASE
            WHEN v_manual_status = 'submitted' THEN v_avg_score::double precision
            ELSE NULL
        END,
        score_decision_type = v_score_decision_type,
        evaluation_status = v_evaluation_status,
        completed_at = v_completed_at,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_video_id;
END;
$$;

COMMIT;
```

## 3. 单评历史数据回填 SQL

这部分用于把现有 `videos.scorer_id` + `manual_evaluations` 的单评历史数据映射为 assignment。

```sql
BEGIN;

------------------------------------------------------------
-- 3.1 老任务和老视频统一标记为单评
------------------------------------------------------------
UPDATE tasks
SET required_review_count = 1
WHERE required_review_count IS DISTINCT FROM 1;

UPDATE videos
SET required_review_count = 1
WHERE required_review_count IS DISTINCT FROM 1;

------------------------------------------------------------
-- 3.2 把旧的 video -> scorer 关系回填成 assignment
------------------------------------------------------------
INSERT INTO video_review_assignments (
    task_id,
    video_id,
    scorer_id,
    review_no,
    review_type,
    status,
    assigned_at,
    started_at,
    submitted_at,
    metadata,
    created_at,
    updated_at
)
SELECT
    v.task_id,
    v.id,
    v.scorer_id,
    1,
    'normal',
    CASE
        WHEN v.manual_status = 'submitted' THEN 'submitted'
        WHEN v.manual_status = 'in_progress' THEN 'in_progress'
        ELSE 'pending'
    END,
    v.assigned_at,
    COALESCE(me.started_at, v.assigned_at),
    CASE
        WHEN v.manual_status = 'submitted'
            THEN COALESCE(me.submitted_at, v.completed_at, v.updated_at)
        ELSE NULL
    END,
    '{}'::jsonb,
    COALESCE(me.created_at, v.created_at),
    GREATEST(COALESCE(me.updated_at, v.updated_at), v.updated_at)
FROM videos v
LEFT JOIN LATERAL (
    SELECT m.*
    FROM manual_evaluations m
    WHERE m.video_id = v.id
      AND m.scorer_id = v.scorer_id
    ORDER BY m.submitted_at DESC NULLS LAST, m.updated_at DESC, m.created_at DESC, m.id DESC
    LIMIT 1
) me ON TRUE
WHERE v.task_id IS NOT NULL
  AND v.scorer_id IS NOT NULL
ON CONFLICT (video_id, scorer_id, review_type) DO NOTHING;

------------------------------------------------------------
-- 3.3 回填最新一条 manual_evaluations.assignment_id
------------------------------------------------------------
WITH ranked_manual AS (
    SELECT
        me.id AS manual_evaluation_id,
        vra.id AS assignment_id,
        ROW_NUMBER() OVER (
            PARTITION BY me.video_id, me.scorer_id
            ORDER BY me.submitted_at DESC NULLS LAST, me.updated_at DESC, me.created_at DESC, me.id DESC
        ) AS rn
    FROM manual_evaluations me
    JOIN video_review_assignments vra
      ON vra.video_id = me.video_id
     AND vra.scorer_id = me.scorer_id
     AND vra.review_no = 1
     AND vra.review_type = 'normal'
)
UPDATE manual_evaluations me
SET assignment_id = rm.assignment_id
FROM ranked_manual rm
WHERE me.id = rm.manual_evaluation_id
  AND rm.rn = 1
  AND me.assignment_id IS NULL;

------------------------------------------------------------
-- 3.4 回填单评汇总字段
------------------------------------------------------------
UPDATE videos
SET submitted_review_count = CASE
        WHEN manual_status = 'submitted' THEN 1
        ELSE 0
    END,
    score_decision_type = CASE
        WHEN manual_status = 'submitted' THEN 'single'
        ELSE NULL
    END;

COMMIT;
```

## 4. 双评分配事务模板

用途：

- 某个任务下的一批视频一次性分配给两个评分员
- 这段 SQL 只适合“还没有为这些视频创建双评 assignment”的场景

使用前请替换：

- `task_id`
- `scorer_a_id`
- `scorer_b_id`
- `video_id` 数组

要求：

- 两个评分员不能相同
- 目标视频必须都属于同一个 `task`

```sql
BEGIN;

WITH params AS (
    SELECT
        'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid AS task_id,
        'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'::uuid AS scorer_a_id,
        'cccccccc-cccc-cccc-cccc-cccccccccccc'::uuid AS scorer_b_id
),
target_videos AS (
    SELECT unnest(ARRAY[
        '11111111-1111-1111-1111-111111111111'::uuid,
        '22222222-2222-2222-2222-222222222222'::uuid
    ]) AS video_id
),
target_reviewers AS (
    SELECT scorer_id, review_no
    FROM params
    CROSS JOIN LATERAL (
        VALUES
            (scorer_a_id, 1::smallint),
            (scorer_b_id, 2::smallint)
    ) AS reviewer(scorer_id, review_no)
)
INSERT INTO video_review_assignments (
    task_id,
    video_id,
    scorer_id,
    review_no,
    review_type,
    status,
    assigned_at,
    started_at,
    submitted_at,
    metadata,
    created_at,
    updated_at
)
SELECT
    p.task_id,
    v.id,
    tr.scorer_id,
    tr.review_no,
    'normal',
    'pending',
    CURRENT_TIMESTAMP,
    NULL,
    NULL,
    '{}'::jsonb,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
FROM params p
JOIN target_videos tv
  ON TRUE
JOIN videos v
  ON v.id = tv.video_id
 AND v.task_id = p.task_id
JOIN target_reviewers tr
  ON TRUE
WHERE NOT EXISTS (
    SELECT 1
    FROM video_review_assignments vra
    WHERE vra.video_id = v.id
      AND vra.review_no = tr.review_no
)
ON CONFLICT (video_id, review_no) DO NOTHING;

UPDATE tasks
SET required_review_count = 2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = (SELECT task_id FROM params);

UPDATE videos
SET scorer_id = NULL,
    assigned_at = CURRENT_TIMESTAMP,
    required_review_count = 2,
    submitted_review_count = 0,
    manual_status = 'pending',
    manual_score = NULL,
    score_decision_type = NULL,
    evaluation_status = CASE
        WHEN ai_status = 'failed' THEN 'failed'
        WHEN ai_status = 'pending' THEN 'pending'
        ELSE 'in_progress'
    END,
    completed_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE id IN (SELECT video_id FROM target_videos);

COMMIT;
```

## 5. 提交人工评分并重算汇总事务模板

用途：

- 某位评分员提交自己的某条 assignment
- upsert 对应的 `manual_evaluations`
- 调用 `refresh_video_manual_summary(...)` 重算 `videos` 的最终人工汇总字段

使用前请替换：

- `assignment_id`
- `rubric_id`
- `total_score`
- `score_details`
- `comments`

```sql
BEGIN;

WITH params AS (
    SELECT
        'dddddddd-dddd-dddd-dddd-dddddddddddd'::uuid AS assignment_id,
        'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'::uuid AS rubric_id,
        86.50::double precision AS total_score,
        '[
          {"itemId":"item-1","score":28,"comment":"动作稳定"},
          {"itemId":"item-2","score":30,"comment":"步骤完整"},
          {"itemId":"item-3","score":28.5,"comment":"收尾较好"}
        ]'::jsonb AS score_details,
        '整体完成较好'::text AS comments
),
target_assignment AS (
    SELECT vra.*
    FROM video_review_assignments vra
    JOIN params p
      ON p.assignment_id = vra.id
    FOR UPDATE
),
updated_assignment AS (
    UPDATE video_review_assignments vra
    SET status = 'submitted',
        started_at = COALESCE(vra.started_at, CURRENT_TIMESTAMP),
        submitted_at = CURRENT_TIMESTAMP,
        updated_at = CURRENT_TIMESTAMP
    FROM target_assignment ta
    WHERE vra.id = ta.id
    RETURNING vra.id, vra.task_id, vra.video_id, vra.scorer_id
)
INSERT INTO manual_evaluations (
    task_id,
    video_id,
    scorer_id,
    rubric_id,
    assignment_id,
    total_score,
    score_details,
    comments,
    status,
    started_at,
    submitted_at,
    created_at,
    updated_at
)
SELECT
    ua.task_id,
    ua.video_id,
    ua.scorer_id,
    p.rubric_id,
    ua.id,
    p.total_score,
    p.score_details,
    p.comments,
    'submitted',
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
FROM updated_assignment ua
JOIN params p
  ON TRUE
ON CONFLICT (assignment_id)
DO UPDATE SET
    rubric_id = EXCLUDED.rubric_id,
    total_score = EXCLUDED.total_score,
    score_details = EXCLUDED.score_details,
    comments = EXCLUDED.comments,
    status = 'submitted',
    started_at = COALESCE(manual_evaluations.started_at, EXCLUDED.started_at),
    submitted_at = EXCLUDED.submitted_at,
    updated_at = CURRENT_TIMESTAMP;

SELECT refresh_video_manual_summary(video_id)
FROM video_review_assignments
WHERE id = 'dddddddd-dddd-dddd-dddd-dddddddddddd'::uuid;

COMMIT;
```

## 6. 双评结果查询示例 SQL

### 6.1 教师 / 学生按视频查看两位评分员结果

```sql
SELECT
    v.id AS video_id,
    v.student_name,
    v.student_number,
    v.manual_score AS final_manual_score,
    v.manual_status AS final_manual_status,
    v.score_decision_type,
    vra.id AS assignment_id,
    vra.review_no,
    vra.scorer_id,
    u.real_name AS scorer_name,
    vra.status AS assignment_status,
    me.total_score AS scorer_manual_score,
    me.score_details,
    me.comments,
    me.submitted_at
FROM videos v
LEFT JOIN video_review_assignments vra
  ON vra.video_id = v.id
 AND vra.review_type = 'normal'
LEFT JOIN users u
  ON u.id = vra.scorer_id
LEFT JOIN manual_evaluations me
  ON me.assignment_id = vra.id
WHERE v.id = '11111111-1111-1111-1111-111111111111'::uuid
ORDER BY vra.review_no ASC;
```

### 6.2 评分员查询自己的 assignment 列表

```sql
SELECT
    vra.id AS assignment_id,
    vra.review_no,
    vra.status,
    vra.assigned_at,
    v.id AS video_id,
    v.student_name,
    v.student_number,
    v.filename,
    t.id AS task_id,
    t.name AS task_name,
    p.id AS project_id,
    p.name AS project_name
FROM video_review_assignments vra
JOIN videos v
  ON v.id = vra.video_id
JOIN tasks t
  ON t.id = vra.task_id
JOIN projects p
  ON p.id = t.project_id
WHERE vra.scorer_id = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'::uuid
  AND vra.review_type = 'normal'
  AND vra.status <> 'cancelled'
ORDER BY vra.assigned_at DESC NULLS LAST, vra.created_at DESC;
```

## 7. 说明

这套 SQL 的职责分层是：

- `video_review_assignments`
  - 存“谁评哪个视频”
- `manual_evaluations`
  - 存“这次评了什么结果”
- `videos`
  - 存“这条视频最终对外展示的人工汇总结果”

因此：

- 学生端可以切换看两位评分员各自结果
- 教师端可以通过下拉框切换查看不同评分员结果
- 任务列表、成绩统计、学生总览页仍然可以直接读取 `videos.manual_score`

如果进入下一阶段要做仲裁：

- 不需要推翻表结构
- 只需要继续使用 `review_type = arbitration`
- 并扩展 `refresh_video_manual_summary(...)` 的汇总规则
