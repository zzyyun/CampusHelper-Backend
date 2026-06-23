-- =====================================================================
-- Campus Helper - Content Service Database Schema
-- 文件名：content_db.sql
-- 作用：内容服务（content-service）独立数据库 campus_content 的初始建表脚本
-- 适用范围：MySQL 8.0+，字符集 utf8mb4
-- 执行方式：
--   mysql -u root -p < content_db.sql
--   或在 Navicat/DataGrip 中直接执行
--
-- ⚠️ 重要约束（每个服务独立数据库）：
--   - content 服务只允许读写 campus_content，绝不跨库
--   - 跨服务关联通过 ID（如 user_id）解耦，禁止使用外键约束
--   - 禁止任何 JOIN 跨服务数据库
-- =====================================================================

CREATE DATABASE IF NOT EXISTS `campus_content`
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

USE `campus_content`;

-- =====================================================================
-- 1. posts 帖子主表
-- 严格对应 docs/content-service-prd.md §3.1
-- 关键索引：
--   - 唯一：id（雪花 ID）
--   - 单列：school_id（多租户隔离必备）
--   - 复合：(school_id, status, id DESC) 用于 ListPosts 高频游标分页
--   - 复合：(school_id, user_id) 用于按用户查询帖子
-- =====================================================================
DROP TABLE IF EXISTS `posts`;
CREATE TABLE `posts` (
  `id`              BIGINT       NOT NULL                    COMMENT '雪花算法生成的帖子ID',
  `school_id`       BIGINT       NOT NULL                    COMMENT '多租户隔离键（不同学校互不可见）',
  `user_id`         BIGINT       NOT NULL                    COMMENT '发帖用户ID',
  `type`            TINYINT      NOT NULL DEFAULT 1          COMMENT '帖子类型 1=通用 2=失物招领 3=二手',
  `title`           VARCHAR(200) NOT NULL                    COMMENT '标题 1-200 字',
  `content`         TEXT         NOT NULL                    COMMENT '正文 1-5000 字',
  `images`          JSON         NULL                        COMMENT '图片 URL 数组（JSON 字符串）',
  `status`          TINYINT      NOT NULL DEFAULT 1          COMMENT '1=pending 2=published 3=expired 4=closed 5=rejected 6=retrieved 7=sold',
  `likes_count`     INT          NOT NULL DEFAULT 0          COMMENT '点赞数（冗余字段，由 post_likes 表触发器或 service 层维护）',
  `comment_count`   INT          NOT NULL DEFAULT 0          COMMENT '评论数（冗余字段）',
  `expired_at`      DATETIME     NULL                        COMMENT '过期时间（自动作废用）',
  -- ─── 业务扩展字段：失物招领 ──────────────────────────────────────
  `lf_type`         TINYINT      NOT NULL DEFAULT 0          COMMENT '1=我丢失 2=我拾到',
  `lf_location`     VARCHAR(200) NOT NULL DEFAULT ''        COMMENT '丢失/拾取地点',
  `lf_contact`      VARCHAR(128) NOT NULL DEFAULT ''        COMMENT '联系方式（私密，禁止索引到 ES）',
  `lf_category`     TINYINT      NOT NULL DEFAULT 0          COMMENT '物品分类 1-8',
  -- ─── 业务扩展字段：二手交易 ──────────────────────────────────────
  `sh_price`        DECIMAL(10,2) NOT NULL DEFAULT 0         COMMENT '期望售价（元）',
  `sh_original_price` DECIMAL(10,2) NOT NULL DEFAULT 0       COMMENT '原价（元）',
  `sh_condition`    TINYINT      NOT NULL DEFAULT 0          COMMENT '成色 1=全新 2=几乎全新 3=良好 4=一般',
  `sh_trade_method` TINYINT      NOT NULL DEFAULT 0          COMMENT '交易方式 1=面交 2=快递',
  `sh_category`     TINYINT      NOT NULL DEFAULT 0          COMMENT '物品分类 1-8',
  -- ─── 时间戳 ──────────────────────────────────────────────────────
  `created_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at`      DATETIME     NULL                        COMMENT '软删除标记',
  PRIMARY KEY (`id`),
  KEY `idx_school_status_id` (`school_id`, `status`, `id` DESC),
  KEY `idx_school_user`      (`school_id`, `user_id`),
  KEY `idx_school_type`      (`school_id`, `type`),
  KEY `idx_deleted_at`       (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='帖子主表：通用 + 失物招领 + 二手';

-- =====================================================================
-- 2. post_likes 帖子点赞表
-- 设计：独立成表而非计数器自增，方便扩展点赞用户列表 / 时间线
-- 唯一约束：(school_id, post_id, user_id) 同一用户对同一帖子只能点赞一次
-- =====================================================================
DROP TABLE IF EXISTS `post_likes`;
CREATE TABLE `post_likes` (
  `id`          BIGINT       NOT NULL                COMMENT '雪花算法生成的点赞ID',
  `school_id`   BIGINT       NOT NULL                COMMENT '多租户隔离键',
  `post_id`     BIGINT       NOT NULL                COMMENT '被点赞的帖子ID',
  `user_id`     BIGINT       NOT NULL                COMMENT '点赞用户ID',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_post_user` (`school_id`, `post_id`, `user_id`),
  KEY `idx_user_created`   (`user_id`, `created_at` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='帖子点赞表';

-- =====================================================================
-- 3. post_comments 帖子评论表
-- 支持一级评论（parent_id=0）和未来二级回复（parent_id=一级评论ID）
-- 软删除：deleted_at 标记，列表查询过滤 deleted_at IS NULL
-- =====================================================================
DROP TABLE IF EXISTS `post_comments`;
CREATE TABLE `post_comments` (
  `id`          BIGINT       NOT NULL                COMMENT '雪花算法生成的评论ID',
  `school_id`   BIGINT       NOT NULL                COMMENT '多租户隔离键',
  `post_id`     BIGINT       NOT NULL                COMMENT '所属帖子ID',
  `user_id`     BIGINT       NOT NULL                COMMENT '评论用户ID',
  `content`     VARCHAR(500) NOT NULL                COMMENT '评论内容 1-500 字',
  `parent_id`   BIGINT       NOT NULL DEFAULT 0      COMMENT '父评论ID，0=一级评论',
  `status`      TINYINT      NOT NULL DEFAULT 1      COMMENT '1=正常 2=已删除',
  `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at`  DATETIME     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_post_created` (`school_id`, `post_id`, `created_at`),
  KEY `idx_user`         (`user_id`),
  KEY `idx_deleted_at`   (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='帖子评论表';

-- =====================================================================
-- 4. 索引优化建议（可选，由 DBA 评估后决定）
-- =====================================================================
-- CREATE INDEX idx_posts_created ON posts(school_id, created_at DESC);
-- ALTER TABLE posts ADD FULLTEXT INDEX ft_title_content (title, content) WITH PARSER ngram;