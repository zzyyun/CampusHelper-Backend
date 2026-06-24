// Package es 提供 Elasticsearch 客户端封装。
//
// 用于 Content Service 的内容搜索与异步同步。
// 提供 Index/Delete/Search 三种基础操作，封装了连接管理和错误处理。
package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// Client Elasticsearch 客户端封装。
type Client struct {
	es    *elasticsearch.Client
	index string // 默认索引名
}

// NewClient 创建 ES 客户端。
// addrs 为 ES 节点地址列表（例: []string{"http://localhost:9200"}）。
func NewClient(addrs []string, index string) (*Client, error) {
	cfg := elasticsearch.Config{
		Addresses: addrs,
	}
	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建 ES 客户端失败: %w", err)
	}

	// Ping 验证连接
	res, err := es.Ping()
	if err != nil {
		log.Printf("[ES] Ping 失败（服务降级运行）: %v", err)
	} else {
		res.Body.Close()
		log.Printf("[ES] 连接成功: %s", strings.Join(addrs, ","))
	}

	return &Client{es: es, index: index}, nil
}

// PostDocument ES 中存储的帖子文档。
type PostDocument struct {
	PostID       int64  `json:"post_id"`
	SchoolID     int64  `json:"school_id"`
	UserID       int64  `json:"user_id"`
	Type         int8   `json:"type"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	Status       int8   `json:"status"`
	LikesCount   int32  `json:"likes_count"`
	CommentCount int32  `json:"comment_count"`
	CreatedAt    int64  `json:"created_at"` // Unix 秒
	UpdatedAt    int64  `json:"updated_at"`
}

// IndexPost 索引（创建或更新）帖子文档。
// docID 使用帖子 ID，支持幂等更新。
func (c *Client) IndexPost(ctx context.Context, doc *PostDocument) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("序列化 ES 文档失败: %w", err)
	}

	docID := fmt.Sprintf("%d", doc.PostID)
	req := esapi.IndexRequest{
		Index:      c.index,
		DocumentID: docID,
		Body:       bytes.NewReader(body),
		Refresh:    "false",
	}

	res, err := req.Do(ctx, c.es)
	if err != nil {
		log.Printf("[ES] 索引文档失败 post=%d: %v", doc.PostID, err)
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("[ES] 索引文档返回错误 post=%d status=%d: %s", doc.PostID, res.StatusCode, res.String())
	}
	return nil
}

// DeletePost 从 ES 删除帖子文档。
func (c *Client) DeletePost(ctx context.Context, postID int64) error {
	docID := fmt.Sprintf("%d", postID)
	req := esapi.DeleteRequest{
		Index:      c.index,
		DocumentID: docID,
	}

	res, err := req.Do(ctx, c.es)
	if err != nil {
		log.Printf("[ES] 删除文档失败 post=%d: %v", postID, err)
		return err
	}
	defer res.Body.Close()

	if res.IsError() && res.StatusCode != 404 {
		log.Printf("[ES] 删除文档返回错误 post=%d status=%d: %s", postID, res.StatusCode, res.String())
	}
	return nil
}

// Search 搜索帖子文档。
// queryJSON 为 ES JSON 查询字符串。
func (c *Client) Search(ctx context.Context, queryJSON string) (*SearchResult, error) {
	req := esapi.SearchRequest{
		Index: []string{c.index},
		Body:  strings.NewReader(queryJSON),
	}

	res, err := req.Do(ctx, c.es)
	if err != nil {
		return nil, fmt.Errorf("ES 搜索失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("ES 搜索错误 status=%d: %s", res.StatusCode, res.String())
	}

	var result SearchResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 ES 搜索结果失败: %w", err)
	}
	return &result, nil
}

// SearchResult ES 搜索结果。
type SearchResult struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []SearchHit `json:"hits"`
	} `json:"hits"`
}

// SearchHit ES 搜索命中条目。
type SearchHit struct {
	ID     string       `json:"_id"`
	Score  float64      `json:"_score"`
	Source PostDocument `json:"_source"`
}

// IsAvailable 检查 ES 是否可用。
func (c *Client) IsAvailable() bool {
	_, err := c.es.Ping()
	return err == nil
}

// 确保 json 序列化可用
var _ = json.NewDecoder
var _ = fmt.Sprintf
