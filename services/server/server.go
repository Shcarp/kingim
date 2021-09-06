package server

import (
	"context"
	"github.com/spf13/cobra"
	"kingim/container"
	"kingim/naming"
	"kingim/naming/consul"
	"kingim/tcp"
	"kingim/services/server/serv"
	"kingim"
	"kingim/logger"
	"kingim/services/server/conf"
	"kingim/services/server/handler"
	"kingim/storage"
	"kingim/wire"
)

type ServerStartOptions struct {
	config string
	serviceName string
}

func NewServerStartCmd(ctx context.Context, version string) *cobra.Command {
	opts := &ServerStartOptions{}

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start a server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunServerStart(ctx, opts, version)
		},
	}
	cmd.PersistentFlags().StringVarP(&opts.config, "config", "c", "./server/conf.yaml", "Config file")
	cmd.PersistentFlags().StringVarP(&opts.serviceName, "serviceName", "s", "chat", "defined a service name,option is login or chat")
	return cmd
}

func RunServerStart(ctx context.Context, opts*ServerStartOptions, version string) error {
	config,err := conf.Init(opts.config)   // 初始化配置文件
	if err != nil {
		return err
	}
	// 初始化 日志系统
	_ = logger.Init(logger.Settings{
		Level: "trace",
	})
	// 初始化路由
	r := kingim.NewRouter()
	// login
	loginHandler := handler.NewLoginHandler()
	r.Handle(wire.CommandLoginSignIn, loginHandler.DoSysLogin)  // 注入方法
	r.Handle(wire.CommandLoginSignOut, loginHandler.DoSysLogout)

	// 初始化Redis
	rdb, err := conf.InitRedis(config.RedisAddrs, "")
	if err != nil {
		return err
	}
	cache := storage.NewRedisStorage(rdb)

	servhandler := serv.NewServHandler(r, cache)
	service := &naming.DefaultService{
		Id: config.ServiceID,
		Name: opts.serviceName,
		Address: config.PublicAddress,
		Port: config.PublicPort,
		Protocol: string(wire.ProtocolTCP),
		Tags: config.Tags,
	}
	srv := tcp.NewServer(config.Listen, service)

	srv.SetReadWait(kingim.DefaultReadWait)
	srv.SetAcceptor(servhandler)
	srv.SetMessageListener(servhandler)
	srv.SetStateListener(servhandler)
	err = container.Init(srv)
	if err != nil {
		return err
	}
	ns,err := consul.NewNaming(config.ConsulURL)
	if err != nil {
		return err
	}
	container.SetServiceNaming(ns)
	return container.Start()
}
