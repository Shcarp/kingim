package websocket

import (
	"fmt"
	"kingim"
	"kingim/logger"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/pkg/errors"
)

type ClientOptions struct {
	HeartBeat time.Duration
	ReadWait  time.Duration
	WriteWait time.Duration
}

type Client struct {
	sync.Mutex
	kingim.Dialer
	once    sync.Once
	id      string
	name    string
	conn    net.Conn
	state   int32
	options ClientOptions
	Meta    map[string]string
}

func NewClient(id, name string, opts ClientOptions) kingim.Client {
	if opts.WriteWait == 0 {
		opts.WriteWait = kingim.DefaultWriteWait
	}
	if opts.ReadWait == 0 {
		opts.ReadWait = kingim.DefaultReadWait
	}
	cli := &Client{
		id:      id,
		name:    name,
		options: opts,
	}
	return cli
}

// 建立连接
func (c *Client) Connect(addr string) error {
	_, err := url.Parse(addr)
	if err != nil {
		return err
	}
	if !atomic.CompareAndSwapInt32(&c.state, 0, 1) { // 比较并且交换，如果开始值为0 就交换 并且放回true 未发生交换则放回false
		return fmt.Errorf("client has connected")
	}
	conn, err := c.Dialer.DialAndHandshake(kingim.DialerContext{
		Id:      c.id,
		Name:    c.name,
		Address: addr,
		Timeout: kingim.DefaultLoginWait,
	})
	if err != nil {
		atomic.CompareAndSwapInt32(&c.state, 1, 0)
		return err
	}
	if conn == nil {
		return fmt.Errorf("conn is nil")
	}
	c.conn = conn
	if c.options.HeartBeat > 0 {
		go func() {
			err := c.heartbealoop(conn) // 开启心跳
			if err != nil {
				logger.Error("heartbealoop stopped ", err)
			}
		}()
	}
	return nil
}

func (c *Client) heartbealoop(conn net.Conn) error {
	tick := time.NewTicker(c.options.HeartBeat)
	for range tick.C {
		if err := c.ping(conn); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) ping(conn net.Conn) error {
	c.Lock()
	defer c.Unlock()
	err := conn.SetWriteDeadline(time.Now().Add(c.options.WriteWait))
	if err != nil {
		return err
	}
	logger.Tracef("%s send ping to server", c.id)
	return wsutil.WriteClientMessage(conn, ws.OpPing, nil)
}

func (c *Client) Read() (kingim.Frame, error) {
	if c.conn == nil {
		return nil, errors.New("connection is nil")
	}
	if c.options.HeartBeat > 0 {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.options.ReadWait))
	}
	frame, err := ws.ReadFrame(c.conn)
	if err != nil {
		return nil, err
	}
	if frame.Header.OpCode == ws.OpClose {
		return nil, errors.New("remote side close the channel")
	}
	return &Frame{
		raw: frame,
	}, nil
}

func (c *Client) Send(payload []byte) error {
	if atomic.LoadInt32(&c.state) == 0 {
		return fmt.Errorf("connection is nil")
	}
	c.Lock()
	defer c.Unlock()
	err := c.conn.SetWriteDeadline(time.Now().Add(c.options.WriteWait))
	if err != nil {
		return err
	}
	return wsutil.WriteClientMessage(c.conn, ws.OpBinary, payload)
}

func (c *Client) ServiceID() string {
	return c.id
}
func (c *Client) ServiceName() string {
	return c.name
}
func (c *Client) GetMeta() map[string]string {
	return c.Meta
}

func (c *Client) SetDialer(dialer kingim.Dialer) {
	c.Dialer = dialer
}
func (c *Client) Close() {
	c.once.Do(func() {
		if c.conn == nil {
			return
		}
		_ = wsutil.WriteClientMessage(c.conn, ws.OpClose, nil)
		c.conn.Close()
		_ = atomic.CompareAndSwapInt32(&c.state, 1, 0)
	})
}
