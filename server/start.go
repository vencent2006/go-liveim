/**
 * @Author: vincent
 * @Description:
 * @File:  start
 * @Version: 1.0.0
 * @Date: 2021/9/3 19:25
 */

package server

import (
	"context"

	"github.com/spf13/cobra"
)

type StartOptions struct {
	id     string // server id
	listen string // listen port
}

func NewServerCmd(ctx context.Context) *cobra.Command {
	opts := &StartOptions{}

	cmd := &cobra.Command{
		Use:   "server",
		Short: "start chat server on websocket",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunServerStart(ctx, opts)
		},
	}

	// set flag
	cmd.PersistentFlags().StringVarP(&opts.id, "serverid", "i", "demo", "server id")
	cmd.PersistentFlags().StringVarP(&opts.listen, "listen", "l", ":8000", "listen address")

	return cmd
}

func RunServerStart(ctx context.Context, opts *StartOptions) error {
	server := NewServer(opts.id, opts.listen)
	defer server.Shutdown()
	return server.Start()
}
