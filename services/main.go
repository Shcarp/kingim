package main

import (
	"context"
	"flag"

	"github.com/spf13/cobra"
	"kingim/logger"
	"kingim/services/gateway"
)

const version = "v1"

func main() {
	flag.Parse()

	root := &cobra.Command{
		Use:     "kingim",
		Version: version,
		Short:   "King IM Cloud",
	}
	ctx := context.Background()

	root.AddCommand(gateway.NewServerStartCmd(ctx, version))
	//root.AddCommand(server.NewServerStartCmd(ctx, version))
	//root.AddCommand(service.NewServerStartCmd(ctx, version))
	//root.AddCommand(router.NewServerStartCmd(ctx, version))

	if err := root.Execute(); err != nil {
		logger.WithError(err).Fatal("Could not run command")
	}
}
