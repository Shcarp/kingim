package serv

import (
	"github.com/golang/protobuf/proto"
	"kingim"
	"kingim/tcp"
	"kingim/wire/pkt"
	"net"
)

type TcpDialer struct {
	ServiceID string
}

func NewDialer(serviceID string) kingim.Dialer {
	return &TcpDialer{
		ServiceID: serviceID,
	}
}

func (d *TcpDialer) DialAndHandshake(ctx kingim.DialerContext) (net.Conn, error) {
	// 1， 拨号建立连接
	conn, err := net.DialTimeout("tcp", ctx.Address, ctx.Timeout)
	if err != nil {
		return nil,err
	}
	// 握手包
	req := &pkt.InnerHandshakeReq{
		ServiceId: d.ServiceID,
	}
	// 使用proto格式化数据
	bts, _ := proto.Marshal(req)
	err = tcp.WriteFrame(conn, kingim.OpBinary,bts)
	if err != nil {
		return nil,err
	}
	return conn,nil
}

