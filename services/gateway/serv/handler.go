package serv

import (
	"bytes"
	"fmt"
	"kingim"
	"kingim/container"
	"kingim/logger"
	"kingim/wire"
	"kingim/wire/pkt"
	"kingim/wire/token"
	"regexp"
	"time"
)

var log = logger.WithFields(logger.Fields{
	"service": "gateway",
	"pkg":     "serv",
})

type Handel struct {
	ServiceID string
}

func (h *Handel) Accept(conn kingim.Conn, duration time.Duration) (string, error) {
	log := logger.WithFields(logger.Fields{
		"ServiceID": h.ServiceID,
		"module":    "Handler",
		"handler":   "Accept",
	})
	log.Infoln("enter")
	// 读取登录包
	_ = conn.SetReadDeadline(time.Now().Add(duration))
	frame, err := conn.ReadFrame()
	if err != nil {
		return "", nil
	}
	buf := bytes.NewBuffer(frame.GetPayload())
	req, err := pkt.MustReadLogicPkt(buf)
	if err != nil {
		return "", err
	}
	// 必须是登录包
	if req.Command == wire.CommandLoginSignIn {
		resp := pkt.NewFrom(&req.Header)
		resp.Status = pkt.Status_InvalidCommand
		_ = conn.WriteFrame(kingim.OpBinary, pkt.Marshal(resp))
		return "", fmt.Errorf("must be a InvalidCommand")
	}
	// 放序列化BODY
	var login pkt.LoginReq
	err = req.ReadBody(&login)
	if err != nil {
		return "", nil
	}
	// token认证
	tk, err := token.Parse(token.DefaultSecret, login.Token)
	if err != nil {
		resp := pkt.NewFrom(&req.Header)
		resp.Status = pkt.Status_Unauthorized
		_ = conn.WriteFrame(kingim.OpBinary, pkt.Marshal(resp))
		return "", err
	}
	id := generateChannelID(h.ServiceID, tk.Account) // 生产全局唯一ID
	req.ChannelId = id
	req.WriteBody(&pkt.Session{
		Account:   tk.Account,
		ChannelId: id,
		GateId:    h.ServiceID,
		App:       tk.App,
		RemoteIP:  getIp(conn.RemoteAddr().String()),
	})
	// 转发给聊天服务
	err = container.Forward(wire.SNLogin, req)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (h *Handel) Receive(ag kingim.Agent, payload []byte) {
	buf := bytes.NewBuffer(payload)
	packet, err := pkt.Read(buf)
	if err != nil {
		return
	}
	// 回应pong  Push -> (wirteloop) writeChan -> writeFrame
	if basePkt, ok := packet.(*pkt.BasicPkt); ok {
		if basePkt.Code == pkt.CodePing {
			_ = ag.Push(pkt.Marshal(&pkt.BasicPkt{Code: pkt.CodePong}))
		}
	}
	// 如果是logicPkt 就转发给逻辑服务处理   container->Froward->ForwardWithSelector->发送信息至选中的服务
	if logicPkt, ok := packet.(*pkt.LogicPkt); ok {
		logicPkt.ChannelId = ag.ID()
		// 更具指令定位服务
		err = container.Forward(logicPkt.ServiceName(), logicPkt)
		if err != nil {
			logger.WithFields(logger.Fields{
				"module": "handler",
				"id":     ag.ID(),
				"cmd":    logicPkt.Command,
				"dest":   logicPkt.Dest,
			}).Error(err)
		}
	}
}

func (h *Handel) Disconnect(id string) error {
	log.Infof("disconnect %s", id)
	logout := pkt.New(wire.CommandLoginSignOut, pkt.WithChannel(id))
	err := container.Forward(wire.SNLogin, logout)
	if err != nil {
		logger.WithFields(logger.Fields{
			"module": "handler",
			"id":     id,
		}).Error(err)
	}
	return nil
}

func generateChannelID(serviceID string, account string) string {
	return fmt.Sprintf("%s_%s_%d", serviceID, account, wire.Seq.Next())
}

var ipExp = regexp.MustCompile(string("\\:[0-9]+$"))

func getIp(addr string) string {
	if addr == "" {
		return ""
	}
	return ipExp.ReplaceAllString(addr, "")
}
