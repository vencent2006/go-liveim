/**
 * @Author: vincent
 * @Description:
 * @File:  server
 * @Version: 1.0.0
 * @Date: 2021/9/3 09:32
 */

package server

import (
	"fmt"
	"go-liveim/util/log"
	"net"
	"net/http"
	"sync"

	"github.com/gobwas/ws/wsutil"

	"github.com/gobwas/ws"
)

const (
	MAX_USER_NUM = 100 // 最大用户数量
)

// websocket server
type Server struct {
	id         string              // 服务器id
	addr       string              // 服务器地址
	sync.Mutex                     // 互斥锁，防止读写网络和操作users（map）的并发安全问题
	once       sync.Once           // 保证close只有一次进入
	users      map[string]net.Conn // 用户连接session
}

// 构造函数
func NewServer(id, addr string) *Server {
	// todo: 关注下sync.Mutex和sync.Once的默认构造行为
	return &Server{
		id:    id,
		addr:  addr,
		users: make(map[string]net.Conn, MAX_USER_NUM), //map必须使用make初始化才能使用
	}
}

/**
 * @Title addUser
 * @Description 添加用户连接session
 * @Param user, 用户名称
 * @Param conn, 用户连接
 * @return old net.Conn;之前该用户存在的conn；if ok == false, old = nil;else old != nil
 * @return ok bool;跟map的返回一样，ok为true标明已经存在了连接，即是old连接；ok为false，old也就为nil了
 **/
func (s *Server) addUser(user string, conn net.Conn) (net.Conn, bool) {
	// lock and defer unlock
	s.Lock()
	defer s.Unlock()

	// 返回旧的链接
	old, ok := s.users[user]
	// 更新为新的连接
	s.users[user] = conn

	return old, ok
}

func (s *Server) delUser(user string) {
	// lock and defer unlock
	s.Lock()
	defer s.Unlock()

	// map operation: delete
	delete(s.users, user)
}

func (s *Server) readLoop(userId string, conn net.Conn) error {
	for {
		frame, err := ws.ReadFrame(conn)
		if err != nil {
			// read frame fail
			return fmt.Errorf("user(%s) ReadFrame fail(err:%v)", userId, err)
		}

		if frame.Header.OpCode == ws.OpClose {
			// client close
			return fmt.Errorf("user(%s) remote side close the conn", userId)
		}

		if frame.Header.OpCode == ws.OpPing {
			// client ping
			log.Logger.Infof("from client(%s): server recv a ping, response a pong", userId)
			err := wsutil.WriteServerMessage(conn, ws.OpPong, nil)
			if err != nil {
				log.Logger.Errorf("server WriteServerMessage to client(%s) fail, err(%v)", userId, err)
			}

			// 继续循环
			continue
		}

		// ready to read frame, firstly need decode mask
		if frame.Header.Masked {
			// exist mask
			ws.Cipher(frame.Payload, frame.Header.Mask, 0)
		}

		// handle text frame
		if frame.Header.OpCode == ws.OpText {
			// OpText
			go s.handle(userId, string(frame.Payload))
		}

	}
}

func (s *Server) handle(userId string, message string) {
	log.Logger.Infof("server recv msg(%s), from %s", message, userId)

	// lock and defer unlock, because
	s.Lock()
	defer s.Unlock()

	// send broadcast msg
	broadcastMsg := fmt.Sprintf("recv %s, from %s", message, userId)
	for user, conn := range s.users {
		if user == userId {
			// same user, not send broadcast msg to itself
			continue
		}

		log.Logger.Infof("send to %s, msg:%s", user, broadcastMsg)

		err := s.writeText(conn, broadcastMsg)
		if err != nil {
			log.Logger.Errorf("write to user(%s) failed, err(%v)", user, err)
		}
	}
}

// write text frame
func (s *Server) writeText(conn net.Conn, message string) error {
	frame := ws.NewTextFrame([]byte(message))
	return ws.WriteFrame(conn, frame)
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 1.upgrade http to websocket
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			log.Logger.Errorf("upgrade websocket fail, err:%v", err)
			_ = conn.Close()
			return
		}

		// 2.get userId from query("user")
		userId := r.URL.Query().Get("user")
		if userId == "" {
			log.Logger.Errorf("get userId from query, fail")
			_ = conn.Close()
			return
		}

		// 3.add user(userId, conn) to session
		oldConn, ok := s.addUser(userId, conn)
		if ok {
			// old connection already existed
			_ = oldConn.Close()
			log.Logger.Infof("user(%s) already connected, kickoff old connection", userId)
		}

		// connected, write log
		log.Logger.Infof("user(%s) upgrade websocket successfully", userId)

		// go func process
		go func(userId string, conn net.Conn) {
			// 4.read loop
			err := s.readLoop(userId, conn)
			if err != nil {
				log.Logger.Errorf("user(%s) readLoop fail(err:%v)", userId, err)
			}

			// 5.disconnect and delete session
			_ = conn.Close()
			s.delUser(userId)

			log.Logger.Infof("user(%s) connection closed", userId)
		}(userId, conn)
	})

	log.Logger.Infof("server started")
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) Shutdown() {
	s.once.Do(func() {
		// lock and defer unlock
		s.Lock()
		defer s.Unlock()

		// close all session
		for i := range s.users {
			_ = s.users[i].Close()
		}
	})
}
