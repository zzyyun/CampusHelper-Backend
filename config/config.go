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
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	Host         string `mapstructure:"host"`
	Port         string `mapstructure:"port"`
	UserDatabase string `mapstructure:"userDatabase"`
	Charset      string `mapstructure:"charset"`
	Driver       string `mapstructure:"driver"`
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
type Config struct {
	Service  map[string]ServiceConfig `mapstructure:"service"`
	Mysql    MysqlConfig              `mapstructure:"mysql"`
	Redis    RedisConfig              `mapstructure:"redis"`
	Jwt      JwtConfig                `mapstructure:"jwt"`
	RabbitMQ RabbitMQConfig           `mapstructure:"rabbitmq"`
	Etcd     EtcdConfig               `mapstructure:"etcd"`
	Jaeger   JaegerConfig             `mapstructure:"jaeger"`
	Wechat   WechatConfig             `mapstructure:"wechat"`
	Gateway  GatewayConfig            `mapstructure:"gateway"`
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
