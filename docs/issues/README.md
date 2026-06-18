# Content Service Issue 索引

> 本目录包含从 [content-service-prd.md](../content-service-prd.md) 拆分出的 GitHub Issue 模板文件。
> 由于当前项目**未初始化为 Git 仓库**且 `gh` CLI **未认证**，Issue 文件以本地 Markdown 形式保存，待环境就绪后可一键导入。

---

## 📋 Issue 清单

### Epic（1 个）

| 编号 | 文件 | 标题 | 优先级 |
|------|------|------|--------|
| Epic | [epic-content-service.md](epic-content-service.md) | Content Service（内容服务）总览 | P1 |

### Sub-Issues（11 个）

| 编号 | 文件 | 标题 | 优先级 | 工期 |
|------|------|------|--------|------|
| #011 | [issue-011-protobuf.md](issue-011-protobuf.md) | Protobuf 接口定义 | P0 | 1-2 天 |
| #001 | [issue-001-post-base.md](issue-001-post-base.md) | 通用帖子基础层 | P0 | 3-4 天 |
| #004 | [issue-004-dfa-filter.md](issue-004-dfa-filter.md) | DFA 敏感词过滤 | P1 | 2-3 天 |
| #005 | [issue-005-review-flow.md](issue-005-review-flow.md) | 内容审核流程 | P1 | 3 天 |
| #006 | [issue-006-list-pagination.md](issue-006-list-pagination.md) | 帖子列表 + 游标分页 | P1 | 2-3 天 |
| #009 | [issue-009-es-sync.md](issue-009-es-sync.md) | ES 异步同步 | P1 | 3-4 天 |
| #010 | [issue-010-search.md](issue-010-search.md) | 内容搜索 | P1 | 2-3 天 |
| #002 | [issue-002-lost-found.md](issue-002-lost-found.md) | 失物招领模板 | P2 | 2 天 |
| #003 | [issue-003-second-hand.md](issue-003-second-hand.md) | 二手交易模板 | P2 | 2 天 |
| #007 | [issue-007-comment-level1.md](issue-007-comment-level1.md) | 一级评论系统 | P2 | 2-3 天 |
| #008 | [issue-008-like-feature.md](issue-008-like-feature.md) | 点赞功能 | P2 | 1-2 天 |

---

## 🔗 依赖关系图

```text
#011 (Protobuf) ─────┬───→ #001 (帖子基础层) ───┬───→ #002 (失物招领)
                     │                          ├───→ #003 (二手交易)
                     │                          ├───→ #006 (列表分页)
                     │                          └───→ #007 (评论)
                     │                                      │
                     └───→ #004 (DFA) ──→ #005 (审核) ─────┴───→ #008 (点赞)
                                                │
                                                ├───→ #009 (ES同步)
                                                └───→ #010 (搜索)
```

### 关键路径（Critical Path）

```
#011 → #001 → #004 → #005 → #009 → #010
```

**总工期估算**：约 14-18 工作日（按 1 人全职开发计算）

### 推荐开发顺序

1. **第一阶段（基础设施）**：#011 → #001
2. **第二阶段（核心功能）**：#004 → #005 → #009 → #010
3. **第三阶段（业务扩展）**：#002 → #003
4. **第四阶段（互动功能）**：#006 → #007 → #008

---

## 🚀 导入到 GitHub 的步骤

### 前置条件

1. **初始化 Git 仓库**：
   ```bash
   cd /mnt/c/go/go_code/src/go_projects/praProject1
   git init
   git add -A
   git commit -m "feat: 初始化 CampusHelper-Backend 项目"
   ```

2. **配置远程仓库**（需先在 GitHub 上创建空仓库）：
   ```bash
   git remote add origin https://github.com/<your-org>/CampusHelper-Backend.git
   git branch -M main
   git push -u origin main
   ```

3. **认证 gh CLI**：
   ```bash
   gh auth login
   ```

### 方案一：使用 gh CLI 批量导入（推荐）

```bash
# 创建 Epic Issue
gh issue create \
  --title "Epic: Content Service（内容服务）" \
  --label "epic:content-service" \
  --body "$(cat docs/issues/epic-content-service.md)"

# 创建 Sub-Issues（按优先级顺序）
for f in docs/issues/issue-*.md; do
  # 从文件名提取标题（如 "issue-011-protobuf.md" → "Protobuf 接口定义"）
  title=$(basename "$f" .md | sed 's/issue-[0-9]*-//' | sed 's/-/ /g')
  gh issue create \
    --title "$title" \
    --label "epic:content-service" \
    --body "$(cat "$f")"
done
```

### 方案二：手动逐个创建

1. 访问 GitHub 仓库的 Issues 页面
2. 点击 "New Issue"
3. 复制对应 Markdown 文件的标题和正文
4. 添加标签 `epic:content-service` 和对应优先级（`P0`/`P1`/`P2`）
5. 创建后手动在 Epic Issue 中引用 Sub-Issue 编号（替换 `#ISSUE_NUMBER` 占位符）

### 方案三：使用 GitHub API + 脚本

```bash
# 使用 GitHub Personal Access Token 通过 API 批量创建
# 详见 GitHub Docs: https://docs.github.com/en/rest/issues
```

---

## 📝 标签规范

建议在 GitHub 仓库中预先创建以下标签：

| 标签 | 颜色 | 用途 |
|------|------|------|
| `epic:content-service` | `#5319e7` | 关联 Content Service Epic |
| `P0` | `#b60205` | 阻塞/紧急 |
| `P1` | `#d93f0b` | 高优 |
| `P2` | `#fbca04` | 中优 |
| `feature` | `#0e8a16` | 新功能 |
| `infra` | `#1d76db` | 基础设施 |
| `security` | `#b60205` | 安全相关 |
| `bug` | `#d73a4a` | Bug 修复 |

**创建命令：**
```bash
gh label create "epic:content-service" --color "5319e7"
gh label create "P0" --color "b60205"
gh label create "P1" --color "d93f0b"
gh label create "P2" --color "fbca04"
gh label create "feature" --color "0e8a16"
gh label create "infra" --color "1d76db"
gh label create "security" --color "b60205"
gh label create "bug" --color "d73a4a"
```

---

## 📊 项目统计

- **总 Issue 数**：12（1 Epic + 11 Sub-Issues）
- **总预估工期**：约 25-30 工作日
- **MVP 关键路径工期**：约 14-18 工作日
- **涉及代码文件数**（预估）：约 35-40 个
- **涉及数据表**：7 张（posts、lost_found_posts、second_hand_posts、comments、post_likes、es 索引）
- **外部依赖**：File Service、Message Service、Admin Service、Redis、RabbitMQ、Elasticsearch、Jaeger

---

## 🔗 相关文档

- [PRD 原文](../content-service-prd.md) — 产品需求文档
- [项目根目录](../../) — CampusHelper-Backend
- [CLAUDE.md](../../CLAUDE.md) — 项目规范说明
- [github-flow skill](../../../.claude/skills/github-flow/) — 本技能说明

---

## 💡 后续步骤

1. ✅ 完成 Git 仓库初始化
2. ✅ 完成 GitHub 远程仓库配置
3. ✅ 完成 gh CLI 认证
4. ✅ 创建标签（labels）
5. ⏳ 导入 Epic Issue
6. ⏳ 导入 11 个 Sub-Issues
7. ⏳ 在 Epic Issue 中关联所有 Sub-Issue
8. ⏳ 创建 Project Board 跟踪进度
9. ⏳ 开始开发（使用 `/gh-issue-implement <编号>` 触发实现工作流）

---

*本索引由 github-flow 技能自动生成，基于 content-service-prd.md v1.0*