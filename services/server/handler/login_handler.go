package handler

import (
	"kingim"
	"kingim/logger"
	"kingim/wire/pkt"
)

type LoginHandler struct {

}

func NewLoginHandler() *LoginHandler {
	return &LoginHandler{}
}

func (h *LoginHandler) DoSysLogin(ctx kingim.Context) {
	var session pkt.Session
	// 序列化信息body
	if err := ctx.ReadBody(&session); err!= nil {
		_ = ctx.RespWithError(pkt.Status_NotImplemented, err)
		return
	}
	logger.Infof("do login of %v", session.String())
	// 判断用户是否已登录
	old,err := ctx.GetLocation(session.Account,"")
	if err != nil && err != kingim.ErrSessionNil {
		_ = ctx.RespWithError(pkt.Status_SystemException, err)
		return
	}
	if old != nil {
		_ = ctx.Dispatch(&pkt.KickoutNotify{
			ChannelId: old.ChannelId,
		},old)
	}
	err = ctx.Add(&session)
	if err != nil {
		_ = ctx.RespWithError(pkt.Status_SystemException, err)
		return
	}
	var resp = &pkt.LoginResp{
		ChannelId: session.ChannelId,
		Account: session.Account,
	}
	_ = ctx.Resp(pkt.Status_Success,resp)
}

func (h*LoginHandler) DoSysLogout(ctx kingim.Context) {
	logger.WithField("func", "DoSysLogout").Infof("do Logout of %s %s ", ctx.Session().GetChannelId(), ctx.Session().GetAccount())
	err := ctx.Delete(ctx.Session().GetAccount(),ctx.Session().GetChannelId())
	if err != nil {
		_ = ctx.RespWithError(pkt.Status_SystemException, err)
		return
	}
	_ = ctx.Resp(pkt.Status_Success, nil)
}