package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"go_projects/praProject1/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var minioClient *minio.Client
var bucketName string
var publicEndpoint string

// InitMinIO 初始化 MinIO 客户端并确保 bucket 存在。
func InitMinIO() error {
	cfg := config.Conf.File.Minio
	useSSL := cfg.UseSSL

	cli, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return fmt.Errorf("初始化 MinIO 客户端失败: %w", err)
	}

	// 确保 bucket 存在
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := cli.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return fmt.Errorf("检查 bucket 失败: %w", err)
	}
	if !exists {
		if err := cli.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("创建 bucket 失败: %w", err)
		}
		log.Printf("[file-service] MinIO bucket 创建成功: %s", cfg.Bucket)
	}

	minioClient = cli
	bucketName = cfg.Bucket
	publicEndpoint = cfg.PublicEndpoint
	log.Printf("[file-service] MinIO 初始化完成: %s, bucket: %s", cfg.Endpoint, cfg.Bucket)
	return nil
}

// UploadObject 上传文件到 MinIO 并返回公网可访问 URL。
// storageKey: 对象键（如 "post/2026/06/abc.jpg"）
// contentType: MIME 类型
// data: 文件内容
func UploadObject(ctx context.Context, storageKey, contentType string, data io.Reader, size int64) (string, error) {
	if minioClient == nil {
		return "", fmt.Errorf("MinIO 未初始化")
	}
	_, err := minioClient.PutObject(ctx, bucketName, storageKey, data, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("上传到 MinIO 失败: %w", err)
	}
	return fmt.Sprintf("%s/%s/%s", publicEndpoint, bucketName, storageKey), nil
}

// DeleteObject 从 MinIO 删除文件。
func DeleteObject(ctx context.Context, storageKey string) error {
	if minioClient == nil {
		return fmt.Errorf("MinIO 未初始化")
	}
	return minioClient.RemoveObject(ctx, bucketName, storageKey, minio.RemoveObjectOptions{})
}