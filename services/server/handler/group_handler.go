package handler

import (
	"kingim"
	"kingim/services/server/service"
	"kingim/wire/pkt"
	"kingim/wire/rpc"
)

type GroupHandler struct {
	groupService service.Group
}
func NewGroupHandler(groupService service.Group) *GroupHandler {
	return &GroupHandler{
		groupService: groupService,
	}
}

func (h*GroupHandler) DoCreate(ctx kingim.Context) {
	var req pkt.GroupCreateReq
	if err := ctx.ReadBody(&req); err != nil {
		_ = ctx.RespWithError(pkt.Status_InvalidPacketBody, err)
		return
	}
	resp, err := h.groupService.Create(ctx.Session().GetApp(), &rpc.CreateGroupReq{
		Name: req.GetName(),
		Avatar: req.GetAvatar(),
		Introduction: req.GetIntroduction(),
		Owner: req.Owner,
		Members: req.Members,
	})
	if err != nil {
		_ = ctx.RespWithError(pkt.Status_SystemException, err)
		return
	}
	// 创建完成之后给群中的成员发送一条信息
	locs, err := ctx.GetLocations(req.GetMembers()...)
	if err != nil && err != kingim.ErrSessionNil {
		_ = ctx.RespWithError(pkt.Status_SystemException, err)
		return
	}
	if (len(locs)) > 0 {
		if err = ctx.Dispatch(&pkt.GroupCreateNotify{
			GroupId: resp.GroupId,
			Members: req.GetMembers(),
		}, locs...); err != nil {
			_= ctx.RespWithError(pkt.Status_SystemException, err)
			return
		}
	}


}