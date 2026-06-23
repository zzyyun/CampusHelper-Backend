# 数据库迁移脚本说明

本目录存放各微服务的数据库初始化/迁移脚本。

## 文件清单

| 文件             | 目标数据库          | 适用服务       | 何时执行           |
| ---------------- | ------------------- | -------------- | ------------------ |
| `content_db.sql` | `campus_content`    | content-service | 部署 content 服务前 |

## 核心原则

1. **每个服务独立数据库**：禁止跨服务 JOIN，所有跨服务关联通过 ID 解耦。
2. **雪花 ID 作为主键**：所有表的主键统一为 BIGINT，不使用自增 AUTO_INCREMENT，避免分布式下 ID 冲突。
3. **多租户隔离**：每张业务表都必须有 `school_id` 列（类型 BIGINT NOT NULL），并建立复合索引 `(school_id, ...)`。
4. **禁止外键**：跨服务之间使用 ID 解耦，不建物理外键约束。
5. **软删除**：重要业务表（posts / post_comments）使用 `deleted_at` 做软删除。

## 执行顺序

```bash
# 1. Content Service 数据库
mysql -u root -p < content_db.sql

# 2. 后续服务（task / message / admin / file 等）按需执行对应脚本
```

## 与 GORM AutoMigrate 的关系

- **开发环境**：service 启动时通过 `db.AutoMigrate(&model.X{})` 自动建表/增字段，便于快速迭代。
- **生产环境**：必须使用本目录的 SQL 脚本显式执行，由 DBA 审核后再上线。
- AutoMigrate 不会删除列/索引，也不会变更已有列的类型；这些 DDL 变更必须写在本目录的迁移文件中。

## 添加新迁移脚本的规范

1. 文件名格式：`<service>_db_<version>.sql`，例如 `content_db_002_add_expired_index.sql`
2. 文件顶部必须包含：
   - 服务名 / 数据库名
   - 变更说明（增加/删除/修改了哪些表/字段/索引）
   - 回滚 SQL（注释保留，不要删除）
3. ALTER TABLE 必须用 `IF NOT EXISTS` 保护（MySQL 8.0+ 支持）