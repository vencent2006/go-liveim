package main

import (
	"context"
	"go-liveim/client"
	"go-liveim/server"
	"go-liveim/util/log"

	"github.com/spf13/cobra"
)

func main() {
	defer log.Logger.Sync()

	log.Init("dev")

	root := &cobra.Command{
		Use:   "chat",
		Short: "chat demo on websocket",
	}

	ctx := context.Background()
	root.AddCommand(server.NewServerCmd(ctx))
	root.AddCommand(client.NewClientCmd(ctx))

	if err := root.Execute(); err != nil {
		log.Logger.Fatalw("could not run command", "err", err)
	}
}
