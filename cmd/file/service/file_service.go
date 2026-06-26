package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"

	file_pb "go_projects/praProject1/PB/pb/file_pb"
	"go_projects/praProject1/cmd/file/model"
	"go_projects/praProject1/cmd/file/repo"
	"go_projects/praProject1/config"

	common_pb "go_projects/praProject1/PB/pb/common_pb"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const serviceName = "file-service"

type FileServiceServer struct {
	file_pb.UnimplementedFileServiceServer
}

// ─── 上传逻辑 ───────────────────────────────────────────────────────────────

// UploadResult 上传结果（gRPC 内部使用，但本服务的上传走 Gateway HTTP，gRPC 不直接调用）
type UploadResult struct {
	FileID      int64
	URL         string
	SizeBytes   int64
	ContentType string
}

// Upload 处理文件上传：校验 → 去重 → MinIO 上传 → DB 持久化。
func Upload(ctx context.Context, schoolID, uploaderID int64, category string, file multipart.File, header *multipart.FileHeader) (*UploadResult, error) {
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "FileService.Upload")
	defer span.End()

	cfg := config.Conf.File
	span.SetAttributes(
		attribute.Int64("user.id", uploaderID),
		attribute.Int64("school.id", schoolID),
		attribute.String("file.name", header.Filename),
		attribute.Int64("file.size", header.Size),
	)

	// 1. 大小校验
	maxSize := int64(cfg.MaxSizeMB) * 1024 * 1024
	if header.Size > maxSize {
		return nil, fmt.Errorf("文件超过 %dMB 限制", cfg.MaxSizeMB)
	}

	// 2. MIME 校验（http.DetectContentType 读文件头）
	head := make([]byte, 512)
	n, _ := file.Read(head)
	file.Seek(0, io.SeekStart)
	contentType := http.DetectContentType(head[:n])
	if !isAllowedType(contentType, cfg.AllowedTypes) {
		return nil, fmt.Errorf("不支持的文件类型: %s", contentType)
	}

	// 3. SHA-256 计算（用于去重）
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("计算 SHA-256 失败: %w", err)
	}
	file.Seek(0, io.SeekStart)
	sha256sum := hex.EncodeToString(hasher.Sum(nil))

	// 4. 去重检查
	existing, err := repo.FindBySHA256(sha256sum)
	if err == nil && existing != nil {
		span.SetAttributes(attribute.Bool("file.deduped", true))
		return &UploadResult{
			FileID:      existing.ID,
			URL:         existing.URL,
			SizeBytes:   existing.SizeBytes,
			ContentType: existing.ContentType,
		}, nil
	}

	// 5. 上传到 MinIO
	storageKey := buildStorageKey(category, sha256sum, contentType)
	uploadURL, err := UploadObject(ctx, storageKey, contentType, file, header.Size)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("上传到 MinIO 失败: %w", err)
	}

	// 6. 持久化元数据
	newFile := &model.File{
		SchoolID:    schoolID,
		UploaderID:  uploaderID,
		Category:    normalizeCategory(category),
		StorageKey:  storageKey,
		URL:         uploadURL,
		ContentType: contentType,
		SizeBytes:   header.Size,
		SHA256:      sha256sum,
	}
	if err := repo.Create(newFile); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("保存文件元数据失败: %w", err)
	}

	return &UploadResult{
		FileID:      newFile.ID,
		URL:         uploadURL,
		SizeBytes:   header.Size,
		ContentType: contentType,
	}, nil
}

// ─── gRPC RPC 实现（GetFile / DeleteFile） ─────────────────────────────────

func (s *FileServiceServer) GetFile(ctx context.Context, req *file_pb.GetFileRequest) (*file_pb.FileInfo, error) {
	if req.FileId <= 0 || req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "参数不合法")
	}
	f, err := repo.GetByID(req.FileId)
	if err != nil {
		if err == repo.ErrNotFound {
			return nil, status.Error(codes.NotFound, "文件不存在")
		}
		return nil, err
	}
	if f.UploaderID != req.UserId {
		return nil, status.Error(codes.PermissionDenied, "仅上传者可查询")
	}
	return toPbFileInfo(f), nil
}

func (s *FileServiceServer) DeleteFile(ctx context.Context, req *file_pb.DeleteFileRequest) (*common_pb.BaseResponse, error) {
	if req.FileId <= 0 || req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "参数不合法")
	}
	if err := repo.SoftDelete(req.FileId, req.UserId); err != nil {
		switch err {
		case repo.ErrForbidden:
			return nil, status.Error(codes.PermissionDenied, "仅上传者可删除")
		case repo.ErrNotFound:
			return nil, status.Error(codes.NotFound, "文件不存在")
		default:
			return nil, err
		}
	}
	return &common_pb.BaseResponse{Code: 0, Message: "已删除"}, nil
}

// ─── 辅助函数 ───────────────────────────────────────────────────────────────

func isAllowedType(contentType string, allowedTypes []string) bool {
	for _, t := range allowedTypes {
		if t == contentType {
			return true
		}
	}
	return false
}

func buildStorageKey(category, sha256sum, contentType string) string {
	now := time.Now()
	ext := extFromContentType(contentType)
	return fmt.Sprintf("%s/%d/%02d/%s%s",
		normalizeCategory(category),
		now.Year(),
		int(now.Month()),
		sha256sum[:16], ext,
	)
}

func extFromContentType(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return path.Ext(contentType)
	}
}

func normalizeCategory(category string) string {
	switch model.FileCategory(category) {
	case model.FileCategoryAvatar, model.FileCategoryPost, model.FileCategoryTask:
		return category
	default:
		return string(model.FileCategoryOther)
	}
}

func toPbFileInfo(f *model.File) *file_pb.FileInfo {
	return &file_pb.FileInfo{
		Id:          f.ID,
		SchoolId:    f.SchoolID,
		UploaderId:  f.UploaderID,
		Category:    f.Category,
		Url:         f.URL,
		ContentType: f.ContentType,
		SizeBytes:   f.SizeBytes,
		Sha256:      f.SHA256,
		CreatedAt:   f.CreatedAt.Unix(),
	}
}

// ─── Trace 透传 ─────────────────────────────────────────────────────────────

func extractTraceFromMeta(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	carrier := make(map[string]string)
	for k, vals := range md {
		if len(vals) > 0 {
			carrier[k] = vals[0]
		}
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagationMapCarrier(carrier))
}

type propagationMapCarrier map[string]string

func (c propagationMapCarrier) Get(key string) string  { return c[key] }
func (c propagationMapCarrier) Set(key, value string)  { c[key] = value }
func (c propagationMapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// unused but ensures import
var _ = strings.TrimSpace