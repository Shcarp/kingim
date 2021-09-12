package kingim

import (
	"github.com/pkg/errors"
	"kingim/logger"
	"sync"
	"time"
)

type ChannelImpl struct {
	sync.Mutex
	id string
	Conn
	writechan chan []byte
	once sync.Once
	writeWait time.Duration
	readWait time.Duration
	closed *Event
}

func NewChannel (id string, conn Conn) Channel {
	log := logger.WithFields(logger.Fields{
		"module": "tcp_channel",
		"id" : id,
	})
	ch := &ChannelImpl{
		id : id,
		Conn: conn,
		writechan: make(chan []byte, 5),
		closed: NewEvent(),
		writeWait: time.Second*10,
	}
	go func() {
		// 开启一个协程 写writechan中的数据
		err := ch.writeloop()
		if err != nil {
			log.Info(err)
		}
	}()
	return ch
}

// 开启Channel 之后会调用一个协程来循环的写数据，如果没有发送数据则一直循环，单用户使用Push先管道内推入信息之后，write取出管道的数据进行WriteFrame发送数据，WriteFrame设置过期时间
// 然后调用Conn内部的写数据方法，然后调用ws的写数据方法

// 写数据
func (ch*ChannelImpl) writeloop() error {
	for {
		select {
		case payload := <-ch.writechan: // 读取数据
			err := ch.WriteFrame(OpBinary, payload)
			if err != nil {
				return err
			}
			charlen := len(ch.writechan)
			for i := 0; i < charlen; i++ {
				payload = <- ch.writechan
				err := ch.WriteFrame(OpBinary, payload)
				if err != nil {
					return err
				}
			}
			// 判断是否满了
			err = ch.Conn.Flush()
			if err != nil {
				return err
			}
		case <-ch.closed.Done():  // 关闭
			return nil
		}
	}
}
/**
将将数据放入writechan 内部   // 发送数据
 */
func (ch *ChannelImpl) Push(payload []byte) error {
	if ch.closed.HasFired() {
		return errors.New("channel has closed")
	}
	ch.writechan <-payload
	return nil
}

// 重写了kim.Conn 中的WriteFrame 方法，增加了写超时逻辑
func (ch*ChannelImpl) WriteFrame(code OpCode, payload []byte) error {
	_ = ch.Conn.SetWriteDeadline(time.Now().Add(ch.writeWait))
	return ch.Conn.WriteFrame(code, payload)
}

func (ch*ChannelImpl) ID() string {
	return ch.id
}
func (ch*ChannelImpl) SetWriteWait(writeWait time.Duration)  {
	if writeWait == 0 {
		return
	}
	ch.writeWait = writeWait
}

func (ch*ChannelImpl) SetReadWait(readWait time.Duration) {
	if readWait == 0 {
		return
	}
	ch.readWait = readWait
}
// 循环读取数据 ，判断OpCode的类型，进行相应的操作   // 通过回调函数将数据返回
func (ch*ChannelImpl) Readloop(lst MessageListener) error {
	// 一次只允许一个线程读取
	ch.Lock()
	defer ch.Unlock()
	log := logger.WithFields(logger.Fields{
		"struct": "ChannelImpl",
		"func" : "Readloop",
		"id" : ch.id,
	})
	for {
		frame,err := ch.ReadFrame()
		if err != nil {
			return err
		}
		if frame.GetOpCode() == OpClose {
			return errors.New("remote side close the channel")
		}
		if frame.GetOpCode() == OpPing {
			log.Trace("recv a ping; resp with a pong")
			_ = ch.WriteFrame(OpPong,nil)
			continue
		}
		payload := frame.GetPayload()
		if len(payload) == 0 {    // 如果数据为空，跳过本次
			continue
		}
		go lst.Receive(ch, payload)
	}
}