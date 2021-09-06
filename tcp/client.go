package tcp

import (
	"fmt"
	"kingim"
	"kingim/logger"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

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
	conn    kingim.Conn
	once    sync.Once
	id      string
	name    string
	state   int32
	options ClientOptions
	Meta    map[string]string
}

func NewClient(id string, name string, opts ClientOptions) kingim.Client {
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

func (c *Client) ServiceID() string {
	return c.id
}
func (c *Client) ServiceName() string {
	return c.id
}
func (c *Client) GetMeta() map[string]string {
	return c.Meta
}
func NewClientWithProps(id, name string, meta map[string]string, opts ClientOptions) kingim.Client {
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
		Meta:    meta,
	}
	return cli
}


func (c *Client) Connect(addr string) error {
	_, err := url.Parse(addr)
	if err != nil {
		return err
	}
	if !atomic.CompareAndSwapInt32(&c.state, 0, 1) {
		return fmt.Errorf("client has connectec")
	}
	rawconn, err := c.Dialer.DialAndHandshake(kingim.DialerContext{
		Id:      c.id,
		Name:    c.name,
		Address: addr,
		Timeout: kingim.DefaultLoginWait,
	})
	if err != nil {
		atomic.CompareAndSwapInt32(&c.state, 1, 0)
		return err
	}
	if rawconn == nil {
		return fmt.Errorf("conn is nil")
	}
	c.conn = NewConn(rawconn)
	if c.options.HeartBeat > 0 {
		go func() {
			err := c.heartbealoop()
			if err != nil {
				logger.WithField("module", "tcp.client").Warn("heartbealoop stopped - ", err)
			}
		}()
	}
	return nil
}

// 设置握手逻辑
func (c *Client) SetDialer(dialer kingim.Dialer) {
	c.Dialer = dialer
}

func (c *Client) Send(payload []byte) error {
	if atomic.LoadInt32(&c.state) == 0 {
		fmt.Errorf("connect is nil")
	}
	c.Lock()
	defer c.Unlock()
	err := c.conn.SetReadDeadline(time.Now().Add(c.options.WriteWait))
	if err != nil {
		return err
	}
	return c.conn.WriteFrame(kingim.OpBinary, payload)
}

func (c *Client) Read() (kingim.Frame, error) {
	if c.conn == nil {
		return nil, errors.New("connection is nil")
	}
	if c.options.HeartBeat > 0 {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.options.ReadWait))
	}
	frame, err := c.conn.ReadFrame()
	if err != nil {
		return nil, err
	}
	if frame.GetOpCode() == kingim.OpClose {
		return nil, errors.New("remote side close the channel")
	}
	return frame, nil
}

func (c *Client) Close() {
	c.once.Do(func() {
		if c.conn == nil {
			return
		}
		_ = c.conn.WriteFrame(kingim.OpClose, nil)
		c.conn.Close()
		atomic.CompareAndSwapInt32(&c.state, 1, 0)
	})
}

func (c *Client) heartbealoop() error {
	tick := time.NewTicker(c.options.HeartBeat)
	for range tick.C {
		if err := c.ping(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) ping() error {
	logger.WithField("module", "tcp.client").Tracef("%s send ping to server", c.id)

	err := c.conn.SetReadDeadline(time.Now().Add(c.options.WriteWait))
	if err != nil {
		return err
	}
	return c.conn.WriteFrame(kingim.OpPing, nil)
}
