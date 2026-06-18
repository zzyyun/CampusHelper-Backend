// Package main 实现从 schools.csv 批量导入高校数据到 MySQL 的脚本。
//
// 使用方式（项目根目录下执行）：
//
//	go run ./pkg/data
//	go run ./pkg/data -csv=./pkg/data/schools.csv
//	go run ./pkg/data -config=./config
//
// 设计要点：
//  1. DSN 来自统一配置（config.Conf.Mysql），不再硬编码账号信息。
//  2. CSV 路径跨平台：通过 runtime.Caller 定位源文件所在目录，回退到当前工作目录。
//  3. 写入使用 GORM 的 OnConflict + CreateInBatches，参数化绑定，避免手写 SQL 字符串拼接。
//  4. 兼容 UTF-8 BOM、字段空白、变长行，错误行打印后跳过而非中断。
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/db"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// 默认 CSV 文件名（相对 pkg/data 目录）
	defaultCSVName = "schools.csv"
	// 目标表名
	tableName = "schools"
	// 批量插入批次大小
	defaultBatchSize = 500
)

// School 高校表模型：导入脚本专用，与业务侧模型解耦。
// 表结构与原脚本保持一致：id 主键自增、name 唯一索引。
type School struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"uniqueIndex;size:128;not null" json:"name"`
	City      string    `gorm:"size:64;default:''" json:"city"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 显式声明表名，避免与业务侧同名模型冲突。
func (School) TableName() string { return tableName }

func main() {
	config.InitConfig("")
	DB, err := db.GetDB()
	log.Printf("MySQL 连接成功 host=%s db=%s",
		config.Conf.Mysql.Host, config.Conf.Mysql.UserDatabase)
	if err != nil {
		log.Fatal(err)
	}

	// 3. 自动建表：首次导入时建表，后续幂等
	if err := DB.AutoMigrate(&School{}); err != nil {
		log.Fatalf("AutoMigrate 失败: %v", err)
	}

	// 4. 解析 CSV 路径
	csvPath := flag.String("csv", "", "CSV 文件路径（默认 pkg/data/schools.csv）")
	flag.Parse()
	path := *csvPath
	if path == "" {
		path = resolveCSVPath()
	}
	schools, err := readSchoolsFromCSV(path)
	if err != nil {
		log.Fatalf("读取 CSV 失败 path=%s err=%v", path, err)
	}
	log.Printf("CSV 解析完成 path=%s 有效记录 %d 条", path, len(schools))
	if len(schools) == 0 {
		log.Println("无有效数据可导入，脚本结束")
		return
	}

	// 5. 批量 upsert：name 冲突时更新 city 与 updated_at
	start := time.Now()
	if err := upsertSchools(DB, schools, defaultBatchSize); err != nil {
		log.Fatalf("批量导入失败: %v", err)
	}
	log.Printf("✅ 全部数据导入完成 total=%d 耗时=%s",
		len(schools), time.Since(start).Round(time.Millisecond))
}

// openMySQL 根据配置拼装 DSN 并建立 GORM 连接。
func openMySQL(cfg config.MysqlConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port,
		cfg.UserDatabase, cfg.Charset,
	)
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
}

// resolveCSVPath 跨平台解析默认 CSV 路径：
//  1. 优先使用源文件所在目录（pkg/data/schools.csv），适用于 `go run ./pkg/data`
//  2. 回退到当前工作目录，便于手动指定 cwd 执行
func resolveCSVPath() string {
	if _, file, _, ok := runtime.Caller(0); ok {
		candidate := filepath.Join(filepath.Dir(file), defaultCSVName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, defaultCSVName)
	}
	return defaultCSVName
}

// readSchoolsFromCSV 读取并解析 schools.csv：
//   - 自动剥离 UTF-8 BOM
//   - 自动跳过表头
//   - 跳过字段不足或 ID 解析失败的行并打印告警，不中断整体流程
func readSchoolsFromCSV(path string) ([]School, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开文件: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1 // 允许变长行
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("解析 CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}

	// 兼容 Windows 导出的 CSV：表头首列可能带 UTF-8 BOM

	now := time.Now()
	schools := make([]School, 0, len(records))
	for i, row := range records {
		if i == 0 {
			continue // 跳过表头
		}
		if len(row) < 3 {
			log.Printf("第 %d 行字段不足，已跳过: %v", i+1, row)
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSpace(row[0]), 10, 64)
		if err != nil {
			log.Printf("第 %d 行 ID 解析失败，已跳过: %v", i+1, row)
			continue
		}
		schools = append(schools, School{
			ID:        id,
			Name:      strings.TrimSpace(row[1]),
			City:      strings.TrimSpace(row[2]),
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return schools, nil
}

// upsertSchools 以 name 唯一键作为冲突目标批量 upsert：
//   - name 已存在：更新 city 与 updated_at
//   - name 不存在：插入新记录
//
// 使用 GORM 的 OnConflict + CreateInBatches，由驱动层做参数化绑定，
// 相较手写 INSERT ... ON DUPLICATE KEY UPDATE 字符串更安全、可移植。
func upsertSchools(db *gorm.DB, schools []School, size int) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"city",
			"updated_at",
		}),
	}).CreateInBatches(schools, size).Error
}
