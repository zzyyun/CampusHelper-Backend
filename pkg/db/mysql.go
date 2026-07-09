package db

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go_projects/praProject1/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// dbs 内部维护的「服务 -> *gorm.DB」映射
// 键：服务名（user / content / task / message / admin / file …）
// 遵循微服务「每个服务独立数据库」原则：每个服务只连接自己的库，绝不跨库访问
var (
	dbs   = make(map[string]*gorm.DB)
	dbMux sync.Mutex
)

// slowQueryThreshold 慢查询阈值，超过此时间的 SQL 将被记录到日志
const slowQueryThreshold = 200 * time.Millisecond

// InitDB 根据 service 名连接对应的数据库并写入 dbs。
// 同一服务重复调用是幂等的（第二次返回缓存实例）。
// service 取值：user / content / task / message / admin / file 等。
func InitDB(service string) (*gorm.DB, error) {
	dbMux.Lock()
	defer dbMux.Unlock()

	if v, ok := dbs[service]; ok && v != nil {
		return v, nil
	}

	dbName := config.Conf.Mysql.DBName(service)
	if dbName == "" {
		return nil, fmt.Errorf("db: service %q 未配置数据库名", service)
	}

	cfg := config.Conf.Mysql
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, dbName, cfg.Charset,
	)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("mysql open %s: %w", dbName, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// 使用可配置的连接池参数，未配置时使用4C8G 环境默认值
	pool := cfg.GetPool()
	sqlDB.SetMaxIdleConns(pool.MaxIdleConns)
	sqlDB.SetMaxOpenConns(pool.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(pool.ConnMaxLifetime) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(pool.ConnMaxIdleTime) * time.Second)

	// 注册慢查询监控回调
	if err := registerSlowQueryLogger(db, service); err != nil {
		return nil, fmt.Errorf("注册慢查询监控失败: %w", err)
	}

	log.Printf("[db] %s 连接池: MaxIdle=%d MaxOpen=%d Lifetime=%ds IdleTime=%ds",
		service, pool.MaxIdleConns, pool.MaxOpenConns, pool.ConnMaxLifetime, pool.ConnMaxIdleTime)

	dbs[service] = db
	return db, nil
}

// registerSlowQueryLogger 注册 GORM Before/After 回调对，记录执行时间超过阈值的慢 SQL
func registerSlowQueryLogger(db *gorm.DB, service string) error {
	// Before 回调：在查询前记录开始时间
	beforeFn := func(db *gorm.DB) {
		if db.Statement != nil {
			db.Statement.Context = context.WithValue(db.Statement.Context, startTimeKey{}, time.Now())
		}
	}
	// After 回调：在查询后计算耗时，超阈值则记录慢查询日志
	afterFn := func(db *gorm.DB) {
		if db.Statement == nil || db.Statement.Context == nil {
			return
		}
		start, ok := db.Statement.Context.Value(startTimeKey{}).(time.Time)
		if !ok {
			return
		}
		elapsed := time.Since(start)
		if elapsed > slowQueryThreshold {
			log.Printf("[SLOW_QUERY] service=%s duration=%v rows=%d sql=%s",
				service, elapsed, db.Statement.RowsAffected, db.Statement.SQL.String())
		}
	}

	// callbackReg 定义回调注册器类型，封装 Before/After 链式调用
	type callbackReg func(string, func(*gorm.DB)) error

	// 注册各操作类型的慢查询监控回调
	registrations := []struct {
		reg  callbackReg // 注册器
		name string      // 回调名（必须唯一）
		fn   func(*gorm.DB)
	}{
		{func(n string, fn func(*gorm.DB)) error {
			return db.Callback().Query().Before("gorm:query").Register(n, fn)
		}, "slow_query:before_query", beforeFn},
		{func(n string, fn func(*gorm.DB)) error {
			return db.Callback().Query().After("gorm:query").Register(n, fn)
		}, "slow_query:after_query", afterFn},
		{func(n string, fn func(*gorm.DB)) error {
			return db.Callback().Create().Before("gorm:create").Register(n, fn)
		}, "slow_query:before_create", beforeFn},
		{func(n string, fn func(*gorm.DB)) error {
			return db.Callback().Create().After("gorm:create").Register(n, fn)
		}, "slow_query:after_create", afterFn},
		{func(n string, fn func(*gorm.DB)) error {
			return db.Callback().Update().Before("gorm:update").Register(n, fn)
		}, "slow_query:before_update", beforeFn},
		{func(n string, fn func(*gorm.DB)) error {
			return db.Callback().Update().After("gorm:update").Register(n, fn)
		}, "slow_query:after_update", afterFn},
		{func(n string, fn func(*gorm.DB)) error {
			return db.Callback().Delete().Before("gorm:delete").Register(n, fn)
		}, "slow_query:before_delete", beforeFn},
		{func(n string, fn func(*gorm.DB)) error {
			return db.Callback().Delete().After("gorm:delete").Register(n, fn)
		}, "slow_query:after_delete", afterFn},
	}

	for _, r := range registrations {
		if err := r.reg(r.name, r.fn); err != nil {
			return fmt.Errorf("注册回调 %s 失败: %w", r.name, err)
		}
	}

	return nil
}

// startTimeKey 用于在 context 中存储查询开始时间
type startTimeKey struct{}

// ─── 各服务的便捷初始化方法（推荐使用，明确语义） ─────────────────────────

// InitUserDB 初始化用户服务数据库连接
func InitUserDB() (*gorm.DB, error) { return InitDB("user") }

// InitContentDB 初始化内容服务数据库连接
func InitContentDB() (*gorm.DB, error) { return InitDB("content") }

// InitMessageDB 初始化消息服务数据库连接
func InitMessageDB() (*gorm.DB, error) { return InitDB("message") }

// InitTaskDB 初始化任务服务数据库连接
func InitTaskDB() (*gorm.DB, error) { return InitDB("task") }

// InitFileDB 初始化文件服务数据库连接
func InitFileDB() (*gorm.DB, error) { return InitDB("file") }

// ─── 访问器（供各服务内部获取连接） ──────────────────────────────────────────

// GetDB 按服务名获取已初始化的连接，未初始化时返回错误
// 推荐业务代码使用 GetUserDB / GetContentDB 等强类型访问器
func GetDB(service string) (*gorm.DB, error) {
	dbMux.Lock()
	defer dbMux.Unlock()
	v, ok := dbs[service]
	if !ok || v == nil {
		return nil, fmt.Errorf("db: service %q 未初始化，请先调用 InitDB(%q)", service, service)
	}
	return v, nil
}

// GetUserDB 获取用户服务数据库连接
func GetUserDB() (*gorm.DB, error) { return GetDB("user") }

// GetContentDB 获取内容服务数据库连接
func GetContentDB() (*gorm.DB, error) { return GetDB("content") }

// GetMessageDB 获取消息服务数据库连接
func GetMessageDB() (*gorm.DB, error) { return GetDB("message") }

// GetTaskDB 获取任务服务数据库连接
func GetTaskDB() (*gorm.DB, error) { return GetDB("task") }

// GetFileDB 获取文件服务数据库连接
func GetFileDB() (*gorm.DB, error) { return GetDB("file") }

// CloseAll 关闭所有数据库连接（用于测试或优雅停机）
func CloseAll() error {
	dbMux.Lock()
	defer dbMux.Unlock()
	var firstErr error
	for name, db := range dbs {
		sqlDB, err := db.DB()
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("db %s get sqlDB: %w", name, err)
			}
			continue
		}
		if err := sqlDB.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("db %s close: %w", name, err)
		}
		delete(dbs, name)
	}
	return firstErr
}
