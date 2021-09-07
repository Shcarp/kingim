package conf

import (
	"encoding/json"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/viper"
	"kingim"
	"kingim/logger"
	"strings"
)

// Config Config
type Config struct {
	ServiceID     string
	ServiceName   string `default:"gateway"`
	Listen        string `default:":8000"`
	PublicAddress string
	PublicPort    int `default:"8000"`
	Tags          []string
	ConsulURL     string
	MonitorPort   int `default:"8001"`
	AppSecret     string
	LogLevel      string `default:"INFO"`
}

func (c Config) String() string {
	bts, _ := json.Marshal(c)
	return string(bts)
}

// Init InitConfig
func Init(file string) (*Config, error) {
	viper.SetConfigFile(file)
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/conf")

	var config Config
	if err := viper.ReadInConfig(); err != nil {
		logger.Warn(err)
	} else {
		if err := viper.Unmarshal(&config); err != nil {
			return nil, err
		}
	}
	err := envconfig.Process("kim", &config)
	if err != nil {
		return nil, err
	}
	if config.ServiceID == "" {
		localIP := kingim.GetLocalIP()
		config.ServiceID = fmt.Sprintf("gate_%s", strings.ReplaceAll(localIP, ".", ""))
	}
	if config.PublicAddress == "" {
		config.PublicAddress = kingim.GetLocalIP()
	}
	logger.Info(config)
	return &config, nil
}
