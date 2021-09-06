package tcp

import (
	"context"
	"fmt"
	"kingim"
	"kingim/logger"
	"github.com/pkg/errors"
	"github.com/segmentio/ksuid"
	"net"
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
	quit *kingim.Event
}
type defaultAcceptor struct {
	kingim.Acceptor
}

func NewServer (listen string, service kingim.ServiceRegistration) kingim.Server {
	return &Server{
		listen: listen,
		ServiceRegistration : service,
		ChannelMap : kingim.NewChannels(100),
		quit: kingim.NewEvent(),
		options: ServerOptions{
			loginwait: kingim.DefaultLoginWait,
			writewait: time.Second*10,
		},
	}
}

func (s *Server) Start() error {
	log := logger.WithFields(logger.Fields{
		"module" : "tcp.server",
		"listen" : s.listen,
		"id" : s.ServiceID(),
	})
	if s.StateListener == nil {
		return fmt.Errorf("StateListener is nil ")
	}
	if s.Acceptor == nil {
		s.Acceptor = new(defaultAcceptor)
	}
	lst,err := net.Listen("tcp", s.listen)
	if err != nil {
		return err
	}
	log.Info("start")
	for {
		// 监听
		rawconn, err := lst.Accept()
		if err != nil {
			rawconn.Close()
			log.Warn(err)
			continue
		}
		// 开启一个协程
		go func(rawconn net.Conn) {
			conn := NewConn(rawconn)
			id,err := s.Accept(conn, s.options.loginwait)
			if err != nil {
				_ = conn.WriteFrame(kingim.OpClose, []byte(err.Error()))
				conn.Close()
				return
			}
			if _, ok := s.Get(id); ok {
				log.Warnf("channel %s existed", id)
				_  = conn.WriteFrame(kingim.OpClose, []byte(err.Error()))
				conn.Close()
			}
			channel := kingim.NewChannel(id, conn)
			channel.SetWriteWait(s.options.writewait)
			channel.SetReadWait(s.options.readwait)
			s.Add(channel)
			log.Info("accept" ,channel)
			err = channel.Readloop(s.MessageListener)
			if err!= nil {
				log.Info(err)
			}
			s.Remove(channel.ID())
			_ = s.Disconnect(channel.ID())
			channel.Close()
		}(rawconn)
		select {
		case <-s.quit.Done():
			return fmt.Errorf("listen exited")
		default:
		}
	}
}

func (s * Server) Shutdown (ctx context.Context) error {
	log := logger.WithFields(logger.Fields{
		"module": "tcp.server",
		"id":     s.ServiceID(),
	})
	s.once.Do(func() {
		defer func() {
			log.Info("shutdown")
		}()
		channels := s.All()
		for _, ch := range channels {
			ch.Close()
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
	})
	return nil
}

func (s*Server) Accept(conn kingim.Conn, timeOut time.Duration) (string, error)  {
	return ksuid.New().String(),nil
}

func (s*Server) SetAcceptor(acceptor kingim.Acceptor)  {
	s.Acceptor = acceptor
}

func (s*Server) SetMessageListener(listener kingim.MessageListener )  {
	s.MessageListener = listener
}

func (s*Server) SetStateListener(stateListen kingim.StateListener)  {
	s.StateListener = stateListen
}

func (s*Server) SetReadWait (time time.Duration) {
	s.options.readwait = time
}
func (s*Server) SetChannelMap (channels kingim.ChannelMap) {
	s.ChannelMap = channels
}

func (s*Server) Push(id string,data []byte) error {
	ch, ok := s.ChannelMap.Get(id)
	if !ok {
		return errors.New("channel no found")
	}
	return ch.Push(data)
}



