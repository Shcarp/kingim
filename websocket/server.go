package websocket

import (
	"context"
	"fmt"
	"github.com/gobwas/ws"
	"github.com/pkg/errors"
	"github.com/segmentio/ksuid"
	"kingim"
	"kingim/logger"
	"net/http"
	"sync"
	"time"
)

type ServerOptions struct {
	loginwait time.Duration
	readwait time.Duration
	writewait time.Duration
}

type Server struct {
	listen string
	kingim.ServiceRegistration
	kingim.ChannelMap
	kingim.Acceptor
	kingim.MessageListener
	kingim.StateListener
	once sync.Once
	options ServerOptions
}

type defaultAcceptor struct {
	kingim.Acceptor
}

func NewServer(listen string, service kingim.ServiceRegistration) kingim.Server {
	return &Server{
		listen: listen,
		ServiceRegistration :service,
		options: ServerOptions{
			loginwait: kingim.DefaultLoginWait,
			readwait: kingim.DefaultReadWait,
			writewait: time.Second * 10,
		},
	}
}

func (s*Server) Start() error {
	mux:= http.NewServeMux()
	log := logger.WithFields(logger.Fields{
		"module":"ws.server",
		"listen" : s.listen,
		"id" : s.ServiceID(),
	})
	if s.Acceptor == nil {
		s.Acceptor = new(defaultAcceptor)
	}
	if s.StateListener == nil {
		return  fmt.Errorf("StateListener is nil")
	}
	// 连接管理器
	if s.ChannelMap == nil {
		s.ChannelMap = kingim.NewChannels(100)
	}
	mux.HandleFunc("/", func (w http.ResponseWriter, r*http.Request) {
		rawconn,_,_,err := ws.UpgradeHTTP(r,w)
		if err != nil {
			resp(w , http.StatusBadRequest, err.Error())
		}
		// 包装 conn
		conn := NewConn(rawconn)
		// 第三步 建立连接
		id,err := s.Accept(conn, s.options.loginwait)
		if err != nil {
			_ = conn.WriteFrame(kingim.OpClose, []byte(err.Error()))
			conn.Close()
			return
		}
		// 已经存在
		if _,ok := s.Get(id); ok {
			log.Warnf("channel %s existed", id)
			_ = conn.WriteFrame(kingim.OpClose,[]byte("channelID is existed"))
			conn.Close()
			return
		}
		channel := kingim.NewChannel(id, conn)
		channel.SetReadWait(s.options.readwait)
		channel.SetWriteWait(s.options.writewait)
		s.Add(channel)
		go func(ch kingim.Channel) {
			err := ch.Readloop(s.MessageListener)
			if err != nil {
				log.Info(err)
			}
			s.Remove(ch.ID())
			err =  s.Disconnect(ch.ID())
			if err != nil {
				log.Warn(err)
			}
			ch.Close()
		}(channel)
	})
	log.Infoln("started")
	return http.ListenAndServe(s.listen, mux)
}

func (s*Server) SetMessageListener(lis kingim.MessageListener)  {
	s.MessageListener = lis
}

func (s*Server) SetStateListener(state kingim.StateListener) {
	s.StateListener = state
}

func (s*Server) SetReadWait(readWait time.Duration) {
	s.options.readwait = readWait
}

func (s*Server) SetChannelMap(channels kingim.ChannelMap) {
	s.ChannelMap =channels
}
func (s*Server) Push(id string, data []byte) error {
	ch,ok := s.ChannelMap.Get(id)
	if !ok {
		return errors.New("channel not found")
	}
	ch.Push(data)
	return nil
}

func (s*Server) Shutdown(ctx context.Context) error {
	log := logger.WithFields(logger.Fields{
		"module": "ws.server",
		"id":     s.ServiceID(),
	})
	s.once.Do(func() {
		defer func() {
			log.Info("shutdown")
		}()
		channels := s.ChannelMap.All()
		if channels != nil {
			for _,ch := range channels{
				ch.Close()
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
		}
	})
	return nil
}


func resp(w http.ResponseWriter, code int, body string) {
	w.WriteHeader(code)
	if body != "" {
		_,_ = w.Write([]byte(body))
	}
	logger.Warnf("response with code:%d %s", code, body)
}

func (s*Server) SetAcceptor(accepter kingim.Acceptor)  {
	s.Acceptor = accepter
}

func (s*Server) Accept(Conn kingim.Conn ,time time.Duration) (string, error)  {
	return ksuid.New().String(),nil
}


