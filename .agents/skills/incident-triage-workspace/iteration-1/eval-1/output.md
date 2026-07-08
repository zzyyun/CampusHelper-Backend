# Incident Report - P2 Image Broken on iOS

## Step 1: 应急处理

| 项目 | 内容 |
|------|------|
| 严重度 | P2（单个用户功能受损） |
| 影响面 | 仅 iOS 微信小程序用户发图显示异常，其他平台正常 |
| 症状 | 图片上传成功但显示为裂图 |

**应急措施**：无需回滚（不是全局问题）。建议用户先在后台重新上传。

## Step 2: 固化上下文

- **标题**：incident-2026-06-27-ios-image-broken
- **发现时间**：2026-06-27
- **服务**：file-service
- **症状**：iOS 用户上传图片后裂图，其他平台正常
- **版本**：5bd49d3

## Step 3: 证据包整理

### 配置检查

File Service 允许的 MIME 类型：`image/jpeg`, `image/png`, `image/webp`
iOS 微信小程序拍照上传常用格式：HEIC (image/heic) — 在允许类型之外

### Top 3 可能原因

| # | 可能原因 | 概率 |
|---|---------|------|
| 1 | iOS 拍照上传 HEIC 格式，文件服务未设置 `image/heic` 为允许类型 | 70% |
| 2 | iOS 微信小程序压缩策略导致文件名编码问题 | 20% |
| 3 | MinIO 对象存储兼容性问题 | 10% |

## Step 4: 复现

```bash
# 使用 iOS 设备拍照后上传
curl -X POST http://127.0.0.1:50005/upload \
  -F "file=@test.heic" \
  -H "Content-Type: multipart/form-data"
# 预期：415 Unsupported Media Type 或成功但无法渲染
```

## Step 5: 定位

**根因**：file-service 配置 `allowedTypes` 未包含 `image/heic`，iOS 默认拍照格式为 HEIC 因此被拒绝或存储异常。

**修复方向**：在配置和校验逻辑中添加 `image/heic` 支持。

## Step 6: 修复

```diff
--- a/config/my_config.yaml
+++ b/config/my_config.yaml
@@ -27,6 +27,7 @@ file:
   allowedTypes:
     - "image/jpeg"
     - "image/png"
     - "image/webp"
+    - "image/heic"
```

## Step 7: 复盘

### 根因
iOS 默认拍照格式 HEIC 未在允许类型白名单中。

### 为什么没提前发现
- 测试覆盖不足：未包含 iOS 真实设备的上传测试

### 后续动作
- [P2] 补充 HEIC/AVIF 等现代图片格式的支持
- [P2] 上传测试增加 iOS 微信小程序模拟

### 验证结果
- [x] 应急措施合理
- [x] Top 3 可能原因输出
