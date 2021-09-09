package router

import (
	"context"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/recover"
	"github.com/kataras/iris/v12/middleware/requestid"
	"github.com/spf13/cobra"
	"hash/crc32"
	"kingim/logger"
	"kingim/services/service/conf"
)

type ServerStartOptions struct {
	config string
}

func NewServerStartCmd(ctx context.Context, version string) *cobra.Command {
	opts := &ServerStartOptions{}

	cmd := &cobra.Command{
		Use:   "router",
		Short: "Start a router",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunServerStart(ctx, opts, version)
		},
	}
	cmd.PersistentFlags().StringVarP(&opts.config, "config", "c", "./router/conf.yaml", "Config file")
	return cmd
}

func RunServerStart(ctx context.Context, opts *ServerStartOptions, version string) error {
	config,err := conf.Init(opts.config)
	if err != nil {
		return err
	}
	_ = logger.Init(logger.Settings{
		Level: "trace",
	})
	ac := conf.MakeAccessLog()
	defer ac.Close()
	app := iris.Default()
	app.UseRouter(recover.New())
	app.UseRouter(ac.Handler)
	app.UseRouter(requestid.New())
	app.Get("/health", func(ctx iris.Context) {
		_,_ = ctx.WriteString("ok")
	})
	return app.Listen(config.Listen, iris.WithOptimizations)
}

func HashCode(key string) uint32 {
	hash32 := crc32.NewIEEE()
	hash32.Write([]byte(key))
	return hash32.Sum32() % 1000
}
