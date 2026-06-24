-- Migration #006: 帖子列表查询性能索引
-- 对应 Issue #6: 帖子列表 + 游标分页
-- 执行方式: mysql -u root -p campus_content < migration_006_indexes.sql

-- 1. 时间倒序游标分页索引（最常用）
-- 覆盖: school_id + status + type + created_at DESC + id DESC
CREATE INDEX IF NOT EXISTS idx_list_time
    ON posts (school_id, status, type, created_at DESC, id DESC);

-- 2. 点赞数倒序游标分页索引
-- 覆盖: school_id + status + type + likes_count DESC + id DESC
CREATE INDEX IF NOT EXISTS idx_list_likes
    ON posts (school_id, status, type, likes_count DESC, id DESC);

-- 3. 分类筛选索引（失物招领 / 二手交易）
CREATE INDEX IF NOT EXISTS idx_lf_category
    ON posts (school_id, status, lf_category, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sh_category
    ON posts (school_id, status, sh_category, created_at DESC);

-- 4. 用户帖子索引（"我的帖子"页）
CREATE INDEX IF NOT EXISTS idx_user_posts
    ON posts (school_id, user_id, created_at DESC);
