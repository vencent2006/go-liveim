/**
 * @Author: vincent
 * @Description:
 * @File:  client
 * @Version: 1.0.0
 * @Date: 2021/9/4 16:20
 */

package client

import (
	"context"
	"errors"
	"fmt"
	"go-liveim/util/log"
	"net"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/gobwas/ws/wsutil"

	"github.com/gobwas/ws"
)

type StartOptions struct {
	addr string // 服务端地址
	user string // 登录用户的id
}

// handler
type handler struct {
	conn      net.Conn      // 连接
	close     chan struct{} // close chan
	recv      chan []byte   // recv chan
	heartbeat time.Duration // 心跳时间
}

func NewHandler(conn net.Conn) *handler {
	return &handler{
		conn:      conn,
		close:     make(chan struct{}, 1),
		recv:      make(chan []byte, 10),
		heartbeat: time.Second * 5,
	}
}

func (h *handler) readLoop() error {
	log.Logger.Info("readLoop started")

	// 要求在指定时间内 heartbeat(50sec) * 3内，读到数据
	err := h.conn.SetReadDeadline(time.Now().Add(h.heartbeat * 3))
	if err != nil {
		log.Logger.Errorf("SetReadDeadline fail, err:%s", err)
		return err
	}

	// start loop
	for {
		frame, err := ws.ReadFrame(h.conn)
		if err != nil {
			log.Logger.Error("ReadFrame failed, ", err)
			return err
		}

		// pong
		if frame.Header.OpCode == ws.OpPong {
			// 重置读取超时时间
			log.Logger.Info("recv a pong...")
			_ = h.conn.SetReadDeadline(time.Now().Add(h.heartbeat * 3))
		}

		// switch OpCode
		switch frame.Header.OpCode {
		case ws.OpPong:
			// 重置读取超时时间
			log.Logger.Info("recv a pong...")
			_ = h.conn.SetReadDeadline(time.Now().Add(h.heartbeat * 3))
		case ws.OpClose:
			// close connection
			return errors.New("remote side close the channel")
		case ws.OpText:
			// 文本消息
			h.recv <- frame.Payload
		}
	}
}

func (h *handler) heartbeatLoop() error {
	log.Logger.Info("heartbeatLoop started")

	// set tick
	tick := time.NewTicker(h.heartbeat)
	for range tick.C {
		// send ping for heartbeat to server
		log.Logger.Debug("send ping to server")
		// 10 second read deadline
		_ = h.conn.SetReadDeadline(time.Now().Add(time.Second * 10))
		if err := wsutil.WriteClientMessage(h.conn, ws.OpPing, nil); err != nil {
			return err
		}
	}
	return nil
}

func connect(addr string) (*handler, error) {
	// check url
	_, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	// conn
	conn, _, _, err := ws.Dial(context.Background(), addr)
	if err != nil {
		return nil, err
	}

	// construct handler
	h := NewHandler(conn)

	// read loop
	go func() {
		err := h.readLoop()
		if err != nil {
			log.Logger.Warn("readLoop return ", err)
		}

		// notify close
		h.close <- struct{}{}
	}()

	// heartbeat loop
	go func() {
		err := h.heartbeatLoop()
		if err != nil {
			log.Logger.Info("heartbeatLoop return ", err)
		}
	}()

	return h, nil
}

func run(ctx context.Context, opts *StartOptions) error {
	addr := fmt.Sprintf("%s?user=%s", opts.addr, opts.user)
	log.Logger.Infof("start to connect %s", addr)

	// 1.connect
	h, err := connect(addr)
	if err != nil {
		return err
	}

	// 2.go handle recv
	go func() {
		for msg := range h.recv {
			log.Logger.Info("receive message: ", string(msg))
		}
	}()

	// 3.loop waiting for close
	for {
		select {
		case <-h.close:
			log.Logger.Info("connection closed")
			return nil
		}
	}
}

func NewClientCmd(ctx context.Context) *cobra.Command {
	opts := &StartOptions{}

	cmd := &cobra.Command{
		Use:   "client",
		Short: "start client",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(ctx, opts)
		},
	}

	cmd.PersistentFlags().StringVarP(&opts.addr, "addr", "a", "ws://127.0.0.1:8000", "server address")
	cmd.PersistentFlags().StringVarP(&opts.user, "user", "u", "", "client user")

	return cmd
}
