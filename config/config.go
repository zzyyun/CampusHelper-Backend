package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

var Conf Config

type ServiceConfig struct {
	Name        string `mapstructure:"name"`
	Address     string `mapstructure:"address"`
	LoadBalance bool   `mapstructure:"loadBalance"` // 键名与字段名驼峰不一致，必须加 mapstructure
}
type MysqlConfig struct {
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	Host            string `mapstructure:"host"`
	Port            string `mapstructure:"port"`
	UserDatabase    string `mapstructure:"userDatabase"`
	ContentDatabase string `mapstructure:"contentDatabase"`
	Charset         string `mapstructure:"charset"`
	Driver          string `mapstructure:"driver"`
	// Databases 通用多服务数据库映射，便于后续扩展（task/message/admin/file）
	// key: 服务名（user/content/task/...），value: 数据库名
	// 如果 map 为空，自动用 UserDatabase/ContentDatabase 等具体字段填充
	Databases map[string]string `mapstructure:"databases"`
	// 连接池配置（可选，未配置时使用 4C8G 环境默认值）
	Pool MysqlPoolConfig `mapstructure:"pool"`
}

// MysqlPoolConfig 数据库连接池参数，按4C8G ECS + RDS 场景调优
type MysqlPoolConfig struct {
	MaxIdleConns    int `mapstructure:"maxIdleConns"`    // 空闲连接数（默认25，保持连接预热）
	MaxOpenConns    int `mapstructure:"maxOpenConns"`    // 最大连接数（默认200，RDS max_connections/服务数）
	ConnMaxLifetime int `mapstructure:"connMaxLifetime"` // 连接最大存活时间（秒，默认3600=1h）
	ConnMaxIdleTime int `mapstructure:"connMaxIdleTime"` // 空闲连接最大存活时间（秒，默认600=10min）
}

// DefaultPoolConfig 4C8G ECS + RDS 推荐默认值
var DefaultPoolConfig = MysqlPoolConfig{
	MaxIdleConns:    25,
	MaxOpenConns:    200,
	ConnMaxLifetime: 3600,
	ConnMaxIdleTime: 600,
}

// GetPool 返回连接池配置，未配置字段使用默认值
func (m MysqlConfig) GetPool() MysqlPoolConfig {
	p := m.Pool
	if p.MaxIdleConns <= 0 {
		p.MaxIdleConns = DefaultPoolConfig.MaxIdleConns
	}
	if p.MaxOpenConns <= 0 {
		p.MaxOpenConns = DefaultPoolConfig.MaxOpenConns
	}
	if p.ConnMaxLifetime <= 0 {
		p.ConnMaxLifetime = DefaultPoolConfig.ConnMaxLifetime
	}
	if p.ConnMaxIdleTime <= 0 {
		p.ConnMaxIdleTime = DefaultPoolConfig.ConnMaxIdleTime
	}
	return p
}

// DBName 返回指定服务对应的数据库名。
// 优先从 Databases map 取，若不存在则回退到具体字段（user/content）。
// 返回空字符串表示未配置。
func (m MysqlConfig) DBName(service string) string {
	if v, ok := m.Databases[service]; ok && v != "" {
		return v
	}
	switch service {
	case "user":
		return m.UserDatabase
	case "content":
		return m.ContentDatabase
	}
	return ""
}

type RedisConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Address  string `mapstructure:"address"`
}
type JwtConfig struct {
	AuthKey        string `mapstructure:"authKey"`        // 必须加 mapstructure
	AccessExpireH  int    `mapstructure:"accessExpireH"`  // 必须加 mapstructure
	RefreshExpireH int    `mapstructure:"refreshExpireH"` // 必须加 mapstructure
}
type RabbitMQConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Address  string `mapstructure:"address"`
}
type ElasticsearchConfig struct {
	Addresses []string `mapstructure:"addresses"`
	Index     string   `mapstructure:"index"`
}
type EtcdConfig struct {
	Address []string `mapstructure:"address"`
}
type JaegerConfig struct {
	Endpoint string `mapstructure:"endpoint"`
}
type WechatConfig struct {
	AppID     string `mapstructure:"appId"`     // 必须加 mapstructure
	AppSecret string `mapstructure:"appSecret"` // 必须加 mapstructure
}
type GatewayConfig struct {
	Address   string  `mapstructure:"address"`
	RateLimit float64 `mapstructure:"rateLimit"` // 必须加 mapstructure
	RateBurst int     `mapstructure:"rateBurst"` // 必须加 mapstructure
}
type MinioConfig struct {
	Endpoint       string `mapstructure:"endpoint"`
	AccessKey      string `mapstructure:"accessKey"`
	SecretKey      string `mapstructure:"secretKey"`
	Bucket         string `mapstructure:"bucket"`
	UseSSL         bool   `mapstructure:"useSSL"`
	PublicEndpoint string `mapstructure:"publicEndpoint"`
}
type FileConfig struct {
	Minio        MinioConfig `mapstructure:"minio"`
	MaxSizeMB    int         `mapstructure:"maxSizeMB"`
	AllowedTypes []string    `mapstructure:"allowedTypes"`
}
type Config struct {
	Service       map[string]ServiceConfig `mapstructure:"service"`
	Mysql         MysqlConfig              `mapstructure:"mysql"`
	Redis         RedisConfig              `mapstructure:"redis"`
	Jwt           JwtConfig                `mapstructure:"jwt"`
	RabbitMQ      RabbitMQConfig           `mapstructure:"rabbitmq"`
	Etcd          EtcdConfig               `mapstructure:"etcd"`
	Jaeger        JaegerConfig             `mapstructure:"jaeger"`
	Wechat        WechatConfig             `mapstructure:"wechat"`
	Gateway       GatewayConfig            `mapstructure:"gateway"`
	Elasticsearch ElasticsearchConfig      `mapstructure:"elasticsearch"`
	File          FileConfig               `mapstructure:"file"`
}

func InitConfig(configPath string) {
	workDir, err := os.Getwd()
	if err != nil {
		panic("failed to get working directory: " + err.Error())
	}
	if configPath == "" {
		configPath = workDir + "/config"
	}
	fmt.Println("loading config from path:", configPath)
	// 1. 使用 viper.New() 创建独立实例，避免全局污染
	v := viper.New()
	v.SetConfigName("my_config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configPath)
	// 建议同时把当前工作目录也加上，增加找到配置文件的概率
	v.AddConfigPath(workDir)
	// 2. 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		panic("failed to read config file: " + err.Error())
	}
	// 3. 反序列化到全局变量 Conf
	if err := v.Unmarshal(&Conf); err != nil {
		panic("failed to unmarshal config: " + err.Error())
	}
	// 4. (可选) 打印关键配置，确认是否读取成功
	fmt.Printf("Config loaded successfully. Mysql Host: %s, Jwt AuthKey: %s\n",
		Conf.Mysql.Host, Conf.Jwt.AuthKey)
}
