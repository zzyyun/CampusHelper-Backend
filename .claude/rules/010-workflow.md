---
description: 日常开发工作流约定
globs: ["**/*"]
---

## Git 分支
- `main` — 生产分支，只接受 PR 合入
- `feat/<issue-id>-<简述>` — 功能分支（如 `feat/89-ai-moderation`）
- `fix/<issue-id>-<简述>` — 修复分支

## 提交规范
- 格式：`<type>(<scope>): <描述>`
- type: feat / fix / docs / refactor / test / chore
- scope: 影响的服务或模块（gateway / user / content / task / message / file）
- 示例：`feat(content): 新增帖子 AI 审核接口`

## Code Review
- 所有 PR 必须至少 1 人 Review 后才能合入
- CI 必须通过（test + lint）才能合入

## CI/CD
- push 到 main 自动触发：测试 → lint → 构建镜像 → 推送到 ACR
- 部署到 ECS 需手动触发 Actions workflow
- 镜像 tag 格式：`v1.0-{服务名}-{短SHA}`

## 依赖管理
- 新增依赖前先确认必要性
- 提交前执行 `go mod tidy`