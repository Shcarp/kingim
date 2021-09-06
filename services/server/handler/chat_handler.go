package handler

import (
	"errors"
	"kingim"
	"kingim/services/server/service"
	"kingim/wire/pkt"
	"kingim/wire/rpc"
	"time"
)

var ErrNoDestination = errors.New("dest is empty")

type ChatHandler struct {
	msgService service.Message
	groupService service.Group
}

func NewChatHandler(message service.Message, group service.Group) *ChatHandler {
	return &ChatHandler{
		msgService: message,
		groupService: group,
	}
}

func (c*ChatHandler) DoUserTalk(ctx kingim.Context) {
	if ctx.Header().Dest == "" {
		_ = ctx.RespWithError(pkt.Status_NoDestination, ErrNoDestination)
	}
	// 解包
	var req pkt.MessageReq
	if err := ctx.ReadBody(&req); err!= nil {
		_ = ctx.RespWithError(pkt.Status_InvalidPacketBody, err)
		return
	}
	// 获取接收方位置信息
	receiver := ctx.Header().GetDest()
	loc, err := ctx.GetLocation(receiver, "")
	if err != nil && err != kingim.ErrSessionNil {
		_ = ctx.RespWithError(pkt.Status_InvalidPacketBody, err)
		return
	}
	// 保存离线信息
	sendTime := time.Now().UnixNano()

	// 如果接收方在线 将信息发送过去
	var messageId int64
	if loc != nil {
		if err = ctx.Dispatch(&pkt.MessagePush{
			MessageId: messageId,
			Type: req.GetType(),
			Body: req.GetBody(),
			Extra: req.GetExtra(),
			Sender: ctx.Session().GetAccount(),    // 发送方
			SendTime: sendTime,
		}, loc);err != nil {
			_ = ctx.RespWithError(pkt.Status_SystemException, err)
			return
		}
	}
	// 给发送方回应一条信息
	err = ctx.Resp(pkt.Status_Success, &pkt.MessageResp{
		MessageId: messageId,
		SendTime: sendTime,
	})
}

func (h *ChatHandler) DoGroupTalk(ctx kingim.Context) {
	if ctx.Header().Dest == "" {
		_ = ctx.RespWithError(pkt.Status_SystemException, ErrNoDestination)
	}
	// 解包
	var req pkt.MessageReq
	if err := ctx.ReadBody(&req); err!= nil {
		_ = ctx.RespWithError(pkt.Status_InvalidPacketBody, err)
		return
	}
	// 获取位置信息
	// 群聊里面的dest是群id
	group := ctx.Header().GetDest()
	sendTime := time.Now().UnixNano()
	// 根据群id来储存离线信息
	resp, err := h.msgService.InsertGroup(ctx.Session().GetApp(), &rpc.InsertMessageReq{
		Sender:   ctx.Session().GetAccount(),
		Dest:     group,
		SendTime: sendTime,
		Message: &rpc.Message{
			Type:  req.GetType(),
			Body:  req.GetBody(),
			Extra: req.GetExtra(),
		},
	})
	// 读取成员列表
	memberResp, err := h.groupService.Members(ctx.Session().GetApp(), &rpc.GroupMembersReq{
		GroupId: group,
	})
	if err != nil {
		_ = ctx.RespWithError(pkt.Status_SystemException, err)
		return
	}
	var members = make([]string, len(memberResp.Users))
	for i, user := range memberResp.Users {
		members[i] = user.Account
	}
	locs, err := ctx.GetLocations(members...)
	if err != nil {
		_ = ctx.RespWithError(pkt.Status_SystemException, err)
		return
	}
	if len(locs) > 0 {
		if err = ctx.Dispatch(&pkt.MessagePush{
			MessageId: resp.MessageId,
			Type: req.GetType(),
			Body: req.GetBody(),
			Extra: req.GetExtra(),
			Sender: ctx.Session().GetAccount(),
			SendTime: sendTime,
		}); err != nil {
			_ = ctx.RespWithError(pkt.Status_SystemException,err)
			return
		}
	}
	_ = ctx.Resp(pkt.Status_Success, &pkt.MessageResp{
		MessageId: resp.MessageId,
		SendTime: sendTime,
	})
}

func (h*ChatHandler) DoTalkAck(ctx kingim.Context) {
	var req pkt.MessageAckReq
	if err := ctx.ReadBody(&req); err!= nil {
		_ = ctx.RespWithError(pkt.Status_InvalidPacketBody, err)
		return
	}
	err := h.msgService.SetAck(ctx.Session().GetApp(), &rpc.AckMessageReq{
		Account:   ctx.Session().GetAccount(),
		MessageId: req.GetMessageId(),
	})
	if err != nil {
		_ = ctx.RespWithError(pkt.Status_SystemException, err)
		return
	}
	_ = ctx.Resp(pkt.Status_Success,nil)
}