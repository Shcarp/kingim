package handler

import (
	"github.com/go-redis/redis/v7"
	"github.com/kataras/iris/v12"
	"kingim/services/service/database"
	"kingim/wire"
	"kingim/wire/rpc"
	"time"

	"gorm.io/gorm"
)

type ServiceHandle struct {
	BaseDb    *gorm.DB
	MessageDb *gorm.DB
	Cache     *redis.Client
	Idgen     *database.IDGenerator
}

func (h*ServiceHandle) InsertUserMessage(c iris.Context) {
	var req rpc.InsertMessageReq
	if err := c.ReadBody(&req);err != nil {
		c.StopWithError(iris.StatusBadRequest, err)
		return
	}
	// 将信息保存到数据库，并且返回信息id
	messageId, err := h.insertUserMessage(&req)
	if err != nil {
		c.StopWithError(iris.StatusInternalServerError, err)
	}
	_,_ = c.Negotiate(&rpc.InsertMessageResp{MessageId: messageId})
}

func (h*ServiceHandle) insertUserMessage(req *rpc.InsertMessageReq) (int64, error) {
	// 雪花算法生产ID
	messageId := h.Idgen.Next().Int64()
	messageCount := database.MessageCount{
		ID: messageId,
		Type: byte(req.Message.Type),
		Body: req.Message.Body,
		Extra: req.Message.Extra,
		SendTime: req.SendTime,
	}
	// 扩散写
	idxs := make([]database.MessageIndex, 2)
	idxs[0] = database.MessageIndex{
		ID: h.Idgen.Next().Int64(),
		MessageID: messageId,
		AccountA: req.Dest,
		AccountB: req.Sender,
		SendTime: req.SendTime,
		Direction: 0,
	}
	idxs[1] = database.MessageIndex{
		ID: h.Idgen.Next().Int64(),
		MessageID: messageId,
		AccountA: req.Sender,
		AccountB: req.Dest,
		SendTime: req.SendTime,
		Direction: 1,
	}
	err := h.MessageDb.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&messageCount).Error; err!= nil {  // 保存信息体
			return err
		}
		if err := tx.Create(&idxs).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0,err
	}
	return messageId,nil
}

func (h*ServiceHandle) InsertGroupMessage(c iris.Context) {
	var req rpc.InsertMessageReq
	if err := c.ReadBody(&req); err!= nil {
		c.StopWithError(iris.StatusBadRequest,err)
		return
	}
	messageId, err := h.insertGroupMessage(&req)
	if err != nil {
		c.StopWithError(iris.StatusInternalServerError, err)
		return
	}
	_,_ = c.Negotiate(&rpc.InsertMessageResp{
		MessageId: messageId,
	})
}

func (h*ServiceHandle) insertGroupMessage(req *rpc.InsertMessageReq) (int64, error) {
	messageID := h.Idgen.Next().Int64()

	var members []database.GroupMember
	// 获取成员列表  ，在群信息中 Dest中保存的是群id
	err := h.BaseDb.Where(&database.GroupMember{Group: req.Dest}).Find(&members).Error
	if err != nil {
		return 0,err
	}
	// 群扩散
	var idxs = make([]database.MessageIndex, len(members))
	for i, m := range members {
		idxs[i] = database.MessageIndex{
			ID: h.Idgen.Next().Int64(),
			MessageID: messageID,
			AccountA: m.Account,
			AccountB: req.Sender,
			Direction: 0,
			Group: m.Group,
			SendTime: req.SendTime,
		}
		if m.Account == req.Sender {
			idxs[i].Direction = 1
		}
	}
	messageCount := database.MessageCount{
		ID: h.Idgen.Next().Int64(),
		Type: byte(req.Message.Type),
		Body: req.Message.Body,
		Extra: req.Message.Extra,
		SendTime: req.SendTime,
	}
	err = h.MessageDb.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&messageCount).Error; err!= nil {
			return err
		}
		if err := tx.Create(&idxs).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return messageID, nil
}

func (h*ServiceHandle) MessageAck(c iris.Context) {
	var req rpc.AckMessageReq
	if err := c.ReadBody(&req); err != nil {
		c.StopWithError(iris.StatusBadRequest, err)
		return
	}
	err := setMessageAck(h.Cache, req.Account, req.MessageId)
	if err != nil {
		c.StopWithError(iris.StatusInternalServerError, err)
		return
	}
}

func setMessageAck(cache *redis.Client, account string, msgId int64) error {
	if msgId == 0 {
		return nil
	}
	// 保存信息id至读索引
	key := database.KeyMessageAckIndex(account)
	return cache.Set(key, msgId, wire.OfflineMessageExpiresIn).Err()
}

func (h*ServiceHandle) GetOfflineMessageIndex(c iris.Context) {
	var req rpc.GetOfflineMessageIndexReq
	if err := c.ReadBody(&req); err != nil {
		c.StopWithError(iris.StatusBadRequest, err)
		return
	}
	msgId := req.MessageId
	start, err := h.getSentTime(req.Account, req.MessageId)
	if err != nil {
		c.StopWithError(iris.StatusInternalServerError, err)
		return
	}
	// 更具信息索引信息id 和时间查找信息索引
	var indexes []*rpc.MessageIndex
	tx := h.MessageDb.Model(&database.MessageIndex{}).Select("send_time", "account_b","direction","message_id","group")
	err = tx.Where("account_a=? and send_time>? and direction=?", req.Account, start, 0).Order("send_time asc").Limit(wire.OfflineSyncIndexCount).Find(&indexes).Error
	if err != nil {
		c.StopWithError(iris.StatusInternalServerError, err)
		return
	}
	// 将信息id保存 // 表示同步到了的信息
	err = setMessageAck(h.Cache, req.Account, msgId)
	if err != nil {
		c.StopWithError(iris.StatusInternalServerError, err)
		return
	}
	_,_ = c.Negotiate(&rpc.GetOfflineMessageIndexResp{
		List: indexes,
	})
}

func (h*ServiceHandle) getSentTime(account string, messageId int64) (int64, error) {
	// 1, 冷启动的情况，从服务端拉取信息索引
	if messageId == 0 {
		key := database.KeyMessageAckIndex(account)
		messageId, _ = h.Cache.Get(key).Int64()   // 如果没有发Ack包则这里就是0
	}
	var start int64
	if messageId > 0 {
		var content database.MessageCount
		// 根据信息Id 读取这条信息发送的时间
		err := h.MessageDb.Select("send_time").First(&content, messageId).Error
		if err != nil {
			// 如果这条信息部存在，则放回前面一条
			start = time.Now().AddDate(0,0,-1).UnixNano()
		}else {
			start = content.SendTime
		}
	}
	// 放回默认的你想信息过期时间
	earliestKeepTime := time.Now().AddDate(0,0,-1).UnixNano()
	if start ==0 || start < earliestKeepTime {
		start = earliestKeepTime
	}
	return start,nil
}

func (h*ServiceHandle) GetOfflineMessageContent(c iris.Context) {
	var req rpc.GetOfflineMessageContentReq
	if err := c.ReadBody(&req); err != nil {
		c.StopWithError(iris.StatusBadRequest, err)
		return
	}
	mlen := len(req.MessageIds)
	// 客户端sdk配合
	if mlen > wire.OfflineSyncIndexCount {
		c.StopWithText(iris.StatusBadRequest, "too many MessageIds")
		return
	}
	var contents []*rpc.Message
	err := h.MessageDb.Model(&database.MessageCount{}).Where(req.MessageIds).Find(&contents).Error
	if err != nil {
		c.StopWithError(iris.StatusInternalServerError, err)
		return
	}
	_,_ =c.Negotiate(&rpc.GetOfflineMessageContentResp{List: contents})
}
