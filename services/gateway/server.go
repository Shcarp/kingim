package gateway

import (
	"context"
	"github.com/spf13/cobra"
	"kingim"
	"kingim/container"
	"kingim/logger"
	"kingim/naming"
	"kingim/naming/consul"
	"kingim/services/gateway/conf"
	"kingim/services/gateway/serv"
	"kingim/websocket"
	"kingim/wire"
	"time"
)

type ServerStartOptions struct {
	config string
	protocol string
}

// 创建一个http指令
func NewServerStartCmd(ctx context.Context, version string) *cobra.Command {
	opts := &ServerStartOptions{}
	cmd := &cobra.Command{
		Use: "gateway",
		Short: "Start a gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunServerStart(ctx, opts, version)
		},
	}
	cmd.PersistentFlags().StringVarP(&opts.config, "config", "c", "./gateway/conf.yaml", "Config file")
	cmd.PersistentFlags().StringVarP(&opts.protocol, "protocol", "p", "ws", "protocol of ws or tcp")
	return cmd
}

func RunServerStart(ctx context.Context, opts*ServerStartOptions, version string) error {
	config ,err := conf.Init(opts.config)
	if err != nil {
		return err
	}
	// 初始化日志
	_ = logger.Init(logger.Settings{
		Level: "trace",
	})
	handler := &serv.Handel{
		ServiceID: config.ServiceID,
	}
	var srv kingim.Server
	service := &naming.DefaultService{
		Id: config.ServiceID,
		Name: config.ServiceName,
		Address: config.PublicAddress,
		Port: config.PublicPort,
		Protocol: opts.protocol,
		Tags: config.Tags,
	}
	if opts.protocol == "ws" {
		srv = websocket.NewServer(config.Listen, service)
	}
	// 将方法注入服务
	srv.SetReadWait(time.Minute*2)
	srv.SetAcceptor(handler)
	srv.SetMessageListener(handler)
	srv.SetStateListener(handler)
	_ = container.Init(srv, wire.SNChat, wire.SNLogin)  // 初始化网关服务，并且注入依赖
	ns, err:= consul.NewNaming(config.ConsulURL)
	if err != nil {
		return err
	}
	container.SetServiceNaming(ns)
	container.SerDialer(serv.NewDialer(config.ServiceID))
	return container.Start()
}