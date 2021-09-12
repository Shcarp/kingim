package conf

import (
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v7"
	"github.com/kataras/iris/v12/middleware/accesslog"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"kingim"
	"kingim/logger"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServiceID     string
	NodeID        int64
	Listen        string `default:":8080"`
	PublicAddress string
	PublicPort    int `default:"8080"`
	Tags          []string
	ConsulURL     string
	RedisAddrs    string
	BaseDb        string
	MessageDb     string
	LogLevel      string `default:"INFO"`
}

func (c Config) String() string {
	bts,_ := json.Marshal(c)
	return string(bts)
}

func Init(file string) (*Config, error) {
	viper.SetConfigFile(file)
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/conf")
	var config Config
	if err := viper.ReadInConfig(); err != nil {
		logger.Warn(err)
		config.ServiceID = "royal01"
		config.Listen = ":8080"
		config.ConsulURL = "127.0.0.1:8500"
		//config.Tags =
		config.BaseDb = "root:Aa123456@tcp(sh-cynosdbmysql-grp-69gahe56.sql.tencentcdb.com:21141)/kim_base?charset=utf8mb4&parseTime=True&loc=Local"
		config.MessageDb = "root:Aa123456@tcp(sh-cynosdbmysql-grp-69gahe56.sql.tencentcdb.com:21141)/kim_message?charset=utf8mb4&parseTime=True&loc=Local"
	} else {
		if err := viper.Unmarshal(&config); err != nil {
			return nil, err
		}
	}
	err := envconfig.Process("kim", &config)
	if err != nil {
		return nil,err
	}
	if config.ServiceID == "" {
		localIP := kingim.GetLocalIP()
		config.ServiceID = fmt.Sprintf("royal_%s", strings.ReplaceAll(localIP, ".", ""))
		arr := strings.Split(localIP, ".")
		if len(arr) == 4 {
			suffix, _ := strconv.Atoi(arr[3])
			config.NodeID = int64(suffix)
		}
	}
	if config.PublicAddress == "" {
		config.PublicAddress = kingim.GetLocalIP()
	}
	logger.Info(config)
	return &config,nil
}

func InitRedis(addr string, pass string) (*redis.Client, error) {
	redisdb := redis.NewClient(&redis.Options{
		Addr: addr,
		Password: pass,
		DialTimeout: time.Second * 5,
		ReadTimeout: time.Second * 5,
		WriteTimeout: time.Second * 5,
	})
	_, err := redisdb.Ping().Result()
	if err != nil {
		logger.Print(err)
		return nil,err
	}
	return redisdb, err
}

func InitFailoverRedis(masterName string, sentinelAddrs []string, password string, timeout time.Duration) (*redis.Client, error) {
	redisdb := redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName: masterName,
		SentinelAddrs: sentinelAddrs,
		Password: password,
		DialTimeout: time.Second * 5,
		WriteTimeout: timeout,
		ReadTimeout: timeout,
	})
	_, err := redisdb.Ping().Result()
	if err != nil {
		logrus.Warn(err)
	}
	return redisdb, err
}

func MakeAccessLog() *accesslog.AccessLog {
	ac := accesslog.File("./access.log")
	ac.AddOutput(os.Stdout)
	ac.Delim = '|'
	ac.TimeFormat = "2006-01-02 15:04:05"
	ac.Async = false
	ac.IP = true
	ac.BytesReceivedBody = true
	ac.BytesSentBody = true
	ac.BytesReceived = false
	ac.BytesSent = false
	ac.BodyMinify = true
	ac.RequestBody = true
	ac.ResponseBody = false
	ac.KeepMultiLineError = true
	ac.PanicLog = accesslog.LogHandler
	return ac
}
