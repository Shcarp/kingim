package kingim

import (
	"kingim/logger"
	"sync"
)

type ChannelMap interface {
	Add(Channel)
	Remove(id string)
	Get(id string) (Channel,bool)
	All() []Channel
}

type ChannelsImpl struct {
	channels *sync.Map
}

func NewChannels(num int) ChannelMap {
	return &ChannelsImpl {
		channels: new(sync.Map),
	}
}

func (ch * ChannelsImpl) Add(channel Channel) {
	if channel.ID() == "" {
		logger.WithFields(logger.Fields{
			"module": "channelsImpl",
		}).Error("channel id is required")
	}
	ch.channels.Store(channel.ID(), channel)
}

func (ch * ChannelsImpl) Remove(id string) {
	ch.channels.Delete(id)
}

func (ch*ChannelsImpl) Get(id string) (Channel,bool) {
	if id == "" {
		logger.WithFields(logger.Fields{
			"module": "ChannelsImpl",
		}).Error("channel id is required")
	}
	val, ok := ch.channels.Load(id)
	if !ok {
		return nil,false
	}
	return val.(Channel), true
}

func (ch*ChannelsImpl) All() []Channel {
	arr := make([]Channel,0)
	ch.channels.Range(func(key, value interface{}) bool {
		arr =  append(arr, value.(Channel))
		return true
	})
	return arr
}