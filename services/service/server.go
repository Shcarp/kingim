package service

import (
	"context"
	"fmt"
	"github.com/kataras/iris/v12"
	"github.com/spf13/cobra"
	"hash/crc32"
	"kingim/logger"
	"kingim/naming"
	"kingim/naming/consul"
	"kingim/services/service/conf"
	"kingim/services/service/database"
	"kingim/services/service/handler"
	"kingim/wire"
)

type ServerStartOptions struct {
	config string
}

func NewServerStartCmd(ctx context.Context, version string) *cobra.Command {
	opts := &ServerStartOptions{}

	cmd := &cobra.Command{
		Use:   "royal",
		Short: "Start a rpc service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunServerStart(ctx, opts, version)
		},
	}
	cmd.PersistentFlags().StringVarP(&opts.config, "config", "c", "./service/conf.yaml", "Config file")
	return cmd
}

func RunServerStart(ctx context.Context, opts *ServerStartOptions, version string) error {
	config, err := conf.Init(opts.config)
	if err != nil {
		return err
	}
	// 日志
	_ = logger.Init(logger.Settings{Level: config.LogLevel, Filename: "./data/royal.log"})
	// database init
	db, err := database.InitMysqlDb(config.BaseDb)
	if err != nil {
		return err
	}
	_ = db.AutoMigrate(&database.Group{}, &database.GroupMember{})
	messageDb, err := database.InitMysqlDb(config.MessageDb)
	if err != nil {
		return err
	}
	_ = messageDb.AutoMigrate(&database.MessageIndex{}, &database.MessageCount{})
	if config.NodeID == 0 {
		config.NodeID = int64(HashCode(config.ServiceID))
	}
	idgen, err := database.NewIDGenerator(config.NodeID)
	if err != nil {
		return err
	}
	rdb ,err := conf.InitRedis(config.RedisAddrs, "")
	if err != nil {
		return err
	}
	ns, err := consul.NewNaming(config.ConsulURL)
	if err != nil {
		return err
	}
	_ = ns.Register(&naming.DefaultService{
		Id: config.ServiceID,
		Name: wire.SNService,
		Address: config.PublicAddress,
		Port: config.PublicPort,
		Protocol: "http",
		Tags: config.Tags,
		Meta: map[string]string{
			consul.KeyHealthURL: fmt.Sprintf("http://%s:%d/health", config.PublicAddress, config.PublicPort),
		},
	})
	defer func() {
		_ = ns.Deregister(config.ServiceID)
	}()
	serviceHandler := handler.ServiceHandle{
		BaseDb: db,
		MessageDb: messageDb,
		Idgen: idgen,
		Cache: rdb,
	}
	ac := conf.MakeAccessLog()
	defer ac.Close()
	app := newApp(&serviceHandler)
	app.UseRouter(ac.Handler)
	app.UseRouter(setAllowedResponses)
	return app.Listen(config.Listen, iris.WithOptimizations)
}

// api 接口
func newApp(handle *handler.ServiceHandle) *iris.Application {
	app := iris.Default()
	app.Get("/health", func(ctx iris.Context) {
		_, _ = ctx.WriteString("ok")
	})
	messageAPI := app.Party("/api/:app/message")
	{
		messageAPI.Post("/user",handle.InsertUserMessage)
		messageAPI.Post("/group", handle.InsertGroupMessage)
		messageAPI.Post("/ack", handle.MessageAck)
	}
	groupAPI := app.Party("/api/:app/group")
	{
		groupAPI.Get("/:id",handle.GroupGet)
		groupAPI.Post("", handle.GroupCreate)
		groupAPI.Post("/member", handle.GroupJoin)
		groupAPI.Delete("/member", handle.GroupQuit)
		groupAPI.Get("/members/:id", handle.GroupMembers)
	}
	offlineAPI := app.Party("/api/:app/offline")
	{
		offlineAPI.Use(iris.Compression)
		offlineAPI.Post("/index",handle.GetOfflineMessageIndex)
		offlineAPI.Post("/content", handle.GetOfflineMessageContent)
	}
	return app
}

func setAllowedResponses(ctx iris.Context){
	ctx.Negotiation().JSON().Protobuf().MsgPack()
	ctx.Negotiation().Accept.JSON()
	ctx.Next()
}

func HashCode(key string) uint32 {
	hash32 := crc32.NewIEEE()
	hash32.Write([]byte(key))
	return hash32.Sum32() % 1000
}