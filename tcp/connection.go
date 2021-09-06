package tcp

import (
	"kingim"
	"io"
	"kingim/wire/endian"
	"net"
)

type Frame struct {
	OpCode kingim.OpCode
	Payload []byte
}

func (f*Frame) SetOpCode(code kingim.OpCode) {
	f.OpCode = code
}
func (f*Frame) SetPayload(payload []byte) {
	f.Payload = payload
}
func (f*Frame) GetOpCode() kingim.OpCode {
	return f.OpCode
}
func (f*Frame) GetPayload() []byte {
	return f.Payload
}

type TcpConn struct {
	net.Conn
}
func NewConn(conn net.Conn) *TcpConn {
	return &TcpConn{
		Conn:conn,
	}
}

func (c*TcpConn) Flush() error {
	return nil
}


func (c*TcpConn) ReadFrame() (kingim.Frame, error) {
	opCode,err := endian.ReadUint8(c.Conn)
	if err != nil {
		return nil, err
	}
	payload, err := endian.ReadBytes(c.Conn)
	if err!= nil {
		return nil, err
	}
	return &Frame{
		OpCode: kingim.OpCode(opCode),
		Payload: payload,
	},nil
}

func (c*TcpConn) WriteFrame(code kingim.OpCode, payload []byte) error {
	return  WriteFrame(c.Conn, code, payload)
}

func WriteFrame (c io.Writer, code kingim.OpCode, payload []byte) error{
	if err := endian.WriteUint8(c, uint8(code)); err != nil {
		return err
	}
	if err := endian.WriteBytes(c, payload); err!= nil {
		return err
	}
	return nil
}





