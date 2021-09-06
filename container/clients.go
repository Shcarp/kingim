package container

import (
	"kingim"
	"kingim/logger"
	"sync"
)

type ClientMap interface {
	Add(client kingim.Client)
	Remove(id string)
	Get(id string) (kingim.Client, bool)
	Service(kvs ...string) []kingim.Service
}
type ClientsImpl struct {
	clients *sync.Map
}

func NewClients(num int) ClientMap {
	return &ClientsImpl{
		clients: new(sync.Map),
	}
}

func (ch *ClientsImpl) Add (client kingim.Client)  {
	if client.ServiceID() == "" {
		logger.WithFields(logger.Fields{
			"module": "ClientsImpl",
		}).Error("client id is required")
	}
	ch.clients.Store(client.ServiceID(), client)
}

func (ch*ClientsImpl) Remove(id string) {
	if id == "" {
		logger.WithFields(logger.Fields{
			"module": "ClientsImpl",
		}).Error("clients id is required")
	}
	ch.clients.Delete(id)
}

func (ch*ClientsImpl) Get(id string) (kingim.Client, bool) {
	if id == "" {
		logger.WithFields(logger.Fields{
			"module": "ClientsImpl",
		}).Error("client id is required")
	}
	val,ok := ch.clients.Load(id)
	if !ok {
		return nil,false
	}
	return val.(kingim.Client),true
}

func (ch *ClientsImpl) Service (kvs ...string) []kingim.Service {
	kvlen := len(kvs)
	if kvlen!= 0 && kvlen  != 2 {
		return nil
	}
	arr := make([]kingim.Service, 0)
	ch.clients.Range(func(key, val interface{}) bool {
		ser := val.(kingim.Service)
		if kvlen > 0 && ser.GetMeta()[kvs[0]] != kvs[1] {
			return true
		}
		arr = append(arr,ser)
		return true
	})
	return arr
}

