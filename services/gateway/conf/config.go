package conf

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/klintcheng/kim/logger"
	"github.com/spf13/viper"
)

type Config struct {
	ServiceID string  `envconfig:"serviceId"`
	ServiceName string `envconfig:"serviceName"`
	Namespace string `envconfig:"namespace"`
	Listen string `envconfig:"listen"`
	PublicAddress string `envconfig:"publicAddress"`
	PublicPort int `envconfig:"publicPort"`
	Tags          []string `envconfig:"tags"`
	ConsulURL     string   `envconfig:"consulURL"`
}

func Init(file string) (*Config, error) {
	viper.SetConfigFile(file)  // 指定配置文件目录
	viper.AddConfigPath(".")  // 添加配置文件搜索路径 在工作目录中查找
	viper.AddConfigPath("/etc/conf")
	if err := viper.ReadInConfig(); err!= nil {
		return nil,fmt.Errorf("config file not found: %w", err)
	}
	// 返序列化
	var config Config
	if err := viper.Unmarshal(&config);err != nil {
		return nil,err
	}
	err := envconfig.Process("", &config)
	if err != nil{
		return nil,err
	}
	logger.Info(config)
	return &config, nil
}