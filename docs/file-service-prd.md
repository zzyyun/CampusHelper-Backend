# 产品需求文档：File Service（文件存储服务）

**版本**：1.0
**日期**：2026-06-26
**作者**：Sarah（产品负责人）
**质量评分**：93/100
**前置依赖**：无（全新服务）

---

## 执行摘要

CampusHelper 平台目前没有统一的文件存储能力：Content Service 的帖子图片字段（`Post.images []string`）虽然已定义，但没有上传链路，只能存 URL 字符串；User Service 的头像字段（`avatar_url`）也是纯 URL 字段；Task Service 未来也会需要图片上传。这造成一个明显的功能缺口——前端用户无法上传任何图片资源。

File Service 填补这个空白：作为独立微服务接收 multipart/form-data 上传请求，存储到 MinIO（S3 兼容的对象存储），返回可访问的 URL。前端调用 File Service 上传，拿到 URL 后再传给其他服务（如 Content Service 创建带图的帖子）。本方案不依赖任何云服务商，本地 MinIO 即可支撑校园规模。

---

## 问题陈述

**当前状态**：

| 痛点 | 影响 |
|------|------|
| 帖子图片字段存在但无上传能力 | 帖子只能是纯文本，用户体验受限 |
| 用户头像只能存 URL | 用户无法自定义头像，只能用默认 |
| 多个服务（Content/User/Task）未来都需要文件存储 | 各服务重复造轮子，且路径规划/大小校验/格式校验散落各处 |
| 无统一的鉴权/MIME/大小校验逻辑 | 风险扩散，难以保证一致性 |

**解决方案**：

新增 File Service（`cmd/file/`），作为**统一的文件存储服务**：
- 接收前端直接上传（绕过 Content/User/Task Service，减少链路）
- MinIO（S3 兼容）作为存储后端
- JWT 鉴权（复用现有 JWT 中间件）
- 格式与大小校验（jpeg/png/webp，5MB/张）
- 返回永久 URL（系统级清理孤立文件）

**业务价值**：
- 解决多服务共同依赖的"文件上传"缺口
- 集中处理格式/MIME/大小校验，降低各服务实现成本
- 为帖子图片、用户头像、任务图片等场景提供统一基础设施

---

## 成功指标

| 指标 | 目标 | 验证方式 |
|------|------|---------|
| 上传接口 P95 | < 500ms（含网络上传时间） | 性能测试 |
| 上传成功率 | ≥ 99% | 监控统计 |
| 重复上传检测率 | 100%（SHA-256 哈希去重） | 单元测试 |
| 非法格式/大小拒绝率 | 100% | 单元测试 |

---

## 用户画像

### 主用户：校园普通用户

- **角色**：在校大学生，发帖/评论/修改头像
- **目标**：上传图片增强自己的内容
- **痛点**：目前无法上传任何图片
- **技术层度**：普通（仅使用小程序）

---

## 用户故事与验收标准

### Story 1：上传帖子图片

**作为** 一名学生
**我想要** 在发帖时上传图片
**以便于** 让帖子内容更丰富

**验收标准：**
- [ ] `POST /api/v1/files/upload` 接收 multipart/form-data 上传
- [ ] 请求包含 `file` 字段（必填）
- [ ] 可选 `category` 字段（用于业务分类：avatar / post / task / other）
- [ ] 鉴权：JWT 登录用户可调用，未登录 → 401
- [ ] 大小校验：> 5MB → 400 错误
- [ ] 格式校验：非 jpeg/png/webp → 415 错误
- [ ] 成功返回 `{"url": "https://...", "file_id": 12345, "size": 1024}`

### Story 2：上传头像

**作为** 一名学生
**我想要** 在个人设置页上传头像
**以便于** 拥有自定义头像

**验收标准：**
- [ ] 调用 `POST /api/v1/files/upload?category=avatar`
- [ ] 头像版本：自动裁剪为正方形（本期不裁剪，仅存原图）
- [ ] 返回的 URL 后续传给 `PUT /api/v1/user/info` 的 `avatar_url` 字段

### Story 3：查看文件信息

**作为** 一名前端开发者
**我想要** 通过文件 ID 查询文件元数据
**以便于** 在删除帖子/用户时核对关联文件

**验收标准：**
- [ ] `GET /api/v1/files/:id` 返回文件元数据
- [ ] 含字段：id、url、size、content_type、category、uploader_id、created_at
- [ ] 仅上传者本人和管理员可查询（本期仅本人）

### Story 4：删除文件

**作为** 文件上传者
**我想要** 删除自己上传的文件
**以便于** 清理不再使用的资源

**验收标准：**
- [ ] `DELETE /api/v1/files/:id` 仅上传者本人可操作
- [ ] 软删除（标记 deleted_at）
- [ ] 物理删除由后台任务处理（30 天后自动清理）

### Story 5：系统级清理孤立文件

**作为** 系统
**我想要** 定时清理未被引用的孤儿文件
**以便于** 节省存储空间

**验收标准：**
- [ ] 后台 goroutine 每天扫描一次
- [ ] 删除条件：`deleted_at IS NOT NULL` 且超过 30 天
- [ ] 物理删除前记录日志（文件 ID、URL、删除时间）

---

## 功能需求

### FR-1：上传接口

**文件**：`cmd/file/handler/upload.go`

**接口**：
```
POST /api/v1/files/upload
Content-Type: multipart/form-data

请求字段：
- file (必填)：图片文件
- category (可选)：avatar | post | task | other，默认 other

响应（成功 200）：
{
  "file_id": 12345,
  "url": "http://minio:9000/campus-files/2026/06/abc.jpg",
  "size": 102400,
  "content_type": "image/jpeg"
}

响应（错误 4xx）：
- 400 参数错误 / 文件过大
- 401 未登录
- 415 不支持的格式
- 500 服务器错误
```

**实现要点**：
- 接收 multipart/form-data，使用 `c.FormFile("file")` 获取
- 校验：大小、MIME 类型（http.DetectContentType）
- 生成 SHA-256 哈希（用于去重）
- 上传到 MinIO：`bucket/category/yyyy/mm/{hash}.{ext}`
- 持久化元数据到 files 表

### FR-2：存储后端（MinIO）

**配置**：
```yaml
file:
  minio:
    endpoint: "127.0.0.1:9000"
    accessKey: "minio"
    secretKey: "minio123"
    bucket: "campus-files"
    useSSL: false
    publicEndpoint: "http://127.0.0.1:9000"  # 返回的 URL 使用此 endpoint
```

**依赖**：`github.com/minio/minio-go/v7`

### FR-3：数据模型

```sql
CREATE TABLE files (
    id              BIGINT PRIMARY KEY,                  -- 雪花 ID
    school_id       BIGINT NOT NULL,                     -- 学校隔离
    uploader_id     BIGINT NOT NULL,                     -- 上传者
    category        VARCHAR(32) DEFAULT 'other',
    storage_key     VARCHAR(255) NOT NULL,               -- MinIO 对象键
    url             VARCHAR(512) NOT NULL,               -- 公网可访问 URL
    content_type    VARCHAR(64) NOT NULL,
    size_bytes      BIGINT NOT NULL,
    sha256          CHAR(64) NOT NULL,                   -- 用于去重
    created_at      DATETIME NOT NULL,
    deleted_at      DATETIME DEFAULT NULL,                -- 软删除
    UNIQUE KEY uk_sha256 (sha256),
    INDEX idx_uploader (uploader_id, created_at DESC),
    INDEX idx_cleanup (deleted_at, created_at)
) ENGINE=InnoDB;
```

**去重逻辑**：
- 计算 SHA-256
- 查询数据库中是否已存在相同 SHA-256
- 存在：复用已有文件，uploader_id 更新为当前用户（用于统计/清理）
- 不存在：上传到 MinIO + 插入记录

### FR-4：其他 API

| 方法 | 路由 | Handler | 说明 |
|------|------|---------|------|
| GET | `/api/v1/files/:id` | GetFile | 查询文件元数据 |
| DELETE | `/api/v1/files/:id` | DeleteFile | 软删除（仅上传者） |

### Out of Scope

- ❌ 图片压缩 / 格式转换（webp 等）
- ❌ 头像自动裁剪
- ❌ 文件秒传（哈希去重记录到数据库但不跳过上传字节）
- ❌ 断点续传
- ❌ 多文件批量上传（前端循环调用单文件接口即可）
- ❌ 视频/音频文件
- ❌ 文件夹管理 / 标签系统
- ❌ CDN 加速（依赖 MinIO 本地）
- ❌ 用户级别的"文件列表"页面（直接用 category + uploader_id 查询即可）

---

## 技术约束

### 性能

- 单文件上传 P95 < 500ms（5MB 以内，本地 MinIO）
- 文件下载通过 MinIO 内网 endpoint（不入 Gateway）

### 安全

- JWT 鉴权（必须登录）
- MIME 校验（防止伪扩展名攻击）
- 大小限制（防止 DoS）
- SHA-256 去重（防止重复占用空间）
- URL 不签名（minio 公开 bucket）

> **说明**：MVP 阶段使用 MinIO 公开 bucket（无需签名 URL）。如需防盗链，可在下期启用预签名 URL。

### 集成

- **MinIO**：对象存储 S3 兼容
- **MySQL**：files 表元数据
- **Gateway**：5 个 HTTP 路由
- **被 Content/User/Task 调用**：通过 URL 字符串（不依赖 RPC 强耦合）

### 技术栈

- Go 1.22+
- GORM v2（AutoMigrate `files` 表）
- MinIO Go Client（`github.com/minio/minio-go/v7`）
- Snowflake ID（复用 `pkg/snowflake`）

---

## MVP 范围与分期

### Phase 1：MVP（本 PRD）

| 模块 | 范围 |
|------|------|
| **基础设施** | MinIO docker-compose 部署 + bucket 初始化 |
| **上传接口** | 单文件 multipart 上传 |
| **格式/大小校验** | jpeg/png/webp + 5MB |
| **去重** | SHA-256 哈希去重 |
| **其他 API** | GetFile / DeleteFile |
| **系统清理** | 后台 goroutine 每天清理 30 天前已软删除 |
| **Gateway** | 5 个 HTTP 路由 |

### Phase 2：增强（下一期）

- 图片压缩 / 缩略图
- 头像自动裁剪
- 预签名 URL / CDN
- 用户文件列表页面
- 视频/音频支持

---

## 风险评估

| 风险 | 概率 | 影响 | 缓解策略 |
|------|------|------|---------|
| MinIO 单点故障 | 中 | 高 | 部署文档要求 MinIO 至少 2 副本 |
| 磁盘写满 | 低 | 高 | 监控 MinIO 容量，到 80% 告警 |
| 上传大文件拖慢 Gateway | 中 | 中 | 限制 5MB；下一步可改为前端直传 MinIO（绕过 Gateway） |
| MIME 绕过 | 低 | 高 | 双重校验：MIME + 文件头魔术数 |

---

## 依赖与阻塞

### 依赖

| 依赖项 | 描述 | 状态 |
|--------|------|------|
| MinIO | `campus-files` bucket（需部署） | 需 docker-compose |
| MySQL | `campus_file` 数据库（需创建） | 需配置 |
| Gateway | JWT 鉴权中间件 | ✅ 已有 |

### 已知阻塞

无。基础设施可一次性部署。

---

## 附录

### 配置变更

`my_config.yaml` 新增：

```yaml
mysql:
  databases:
    file: "campus_file"

service:
  file:
    name: "file-service"
    address: "127.0.0.1:50005"
    loadBalance: false

file:
  minio:
    endpoint: "127.0.0.1:9000"
    accessKey: "minio"
    secretKey: "minio123"
    bucket: "campus-files"
    useSSL: false
    publicEndpoint: "http://127.0.0.1:9000"
  maxSizeMB: 5
  allowedTypes:
    - image/jpeg
    - image/png
    - image/webp
```

### MinIO 部署

`deployments/docker/minio-docker-compose.yaml`：
```yaml
version: '3.8'
services:
  minio:
    image: minio/minio:latest
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: minio
      MINIO_ROOT_PASSWORD: minio123
    command: server /data --console-address ":9001"
    volumes:
      - minio-data:/data
volumes:
  minio-data:
```

### Proto 概要

```protobuf
service FileService {
  rpc GetFile (GetFileRequest) returns (FileInfo);
  rpc DeleteFile (DeleteFileRequest) returns (pb.BaseResponse);
}

message FileInfo {
  int64 id = 1;
  string url = 2;
  string content_type = 3;
  int64 size = 4;
  string category = 5;
  int64 uploader_id = 6;
  int64 created_at = 7;
}
```

> **注**：上传接口（POST /api/v1/files/upload）走 Gateway HTTP 路由，不通过 gRPC——因为 HTTP multipart 文件上传直接通过 Gin 处理更高效，无需经过 gRPC 序列化。

### 跨服务集成示例

**Content Service 发帖 + 图片：**
```js
// 前端
const formData = new FormData();
formData.append('file', imageFile);
const { url } = await fetch('/api/v1/files/upload', { method: 'POST', body: formData })
  .then(r => r.json());

// 然后用 url 创建帖子
await fetch('/api/v1/content/posts', {
  method: 'POST',
  body: JSON.stringify({ title, content, images: [url] })
});
```

### 参考文档

- **MinIO Go SDK**：https://github.com/minio/minio-go
- **GORM**：https://gorm.io/docs/
- **Content Service PRD**：`docs/content-service-prd.md`（消费 images 字段）
- **Task Service PRD**：`docs/task-service-prd.md`（未来图片需求）

---

*本 PRD 通过 2 轮迭代式需求对话生成，质量评分 93/100。File Service 采用直传 + MinIO + 哈希去重的轻量架构，集中处理多服务共同依赖的文件存储需求。*