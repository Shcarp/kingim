package storage

import (
	"fmt"
	"github.com/go-redis/redis/v7"
	"github.com/golang/protobuf/proto"
	"kingim"
	"kingim/wire/pkt"
	"time"
)

const (
	LocationExpired = time.Hour*48
)
type RedisStorage struct {
	cli *redis.Client
}

func NewRedisStorage(cli*redis.Client) kingim.SessionStorage {
	return &RedisStorage{
		cli: cli,
	}
}
func (r*RedisStorage) Add(session *pkt.Session) error {
	loc := kingim.Location{
		ChannelId: session.ChannelId,
		GateId: session.GateId,
	}
	locKey := KeyLocation(session.Account, session.Device)
	err := r.cli.Set(locKey, loc.Bytes(), LocationExpired).Err()
	if err != nil {
		return err
	}
	SesKey := KeySession(session.ChannelId)
	buf,_ := proto.Marshal(session)
	err = r.cli.Set(SesKey, buf,LocationExpired).Err()
	if err!= nil {
		return err
	}
	return nil
}

func (r*RedisStorage) Delete(account string, channelId string) error {
	lockey := KeyLocation(account, "")
	err := r.cli.Del(lockey).Err()
	if err != nil {
		return err
	}
	snKey := KeySession(channelId)
	err = r.cli.Del(snKey).Err()
	if err != nil {
		return err
	}
	return nil
}

func (r*RedisStorage) Get(channelId string) (*pkt.Session, error) {
	snKey := KeySession(channelId)
	bts,err := r.cli.Get(snKey).Bytes()
	if err != nil {
		if err != redis.Nil {
			return nil,kingim.ErrSessionNil
		}
		return nil,err
	}
	var session pkt.Session
	_ = proto.Unmarshal(bts, &session)
	return &session,nil
}

func (r*RedisStorage) GetLocation(account string, device string) (*kingim.Location, error) {
	LocKey := KeyLocation(account,"")
	bts,err := r.cli.Get(LocKey).Bytes()
	if err != nil {
		if err != redis.Nil {
			return nil,kingim.ErrSessionNil
		}
		return nil,err
	}
	var loc kingim.Location
	_ = loc.Unmarshal(bts)
	return &loc, nil
}

func (r*RedisStorage) GetLocations(account ...string) ([]*kingim.Location, error) {
	Keys := KeyLocations(account...)
	list,err := r.cli.MGet(Keys...).Result()
	if err != nil {
		return nil,err
	}
	var result = make([]*kingim.Location,0)
	for _,l := range list {
		if l == nil {
			continue
		}
		var loc kingim.Location
		_ = loc.Unmarshal([]byte(l.(string)))
		result = append(result, &loc)
	}
	if len(result) == 0 {
		return nil,kingim.ErrSessionNil
	}
	return result, nil
}

func KeySession(channel string) string {
	return fmt.Sprintf("login:sn:%s", channel)
}


func KeyLocation(account,device string) string {
	if device == "" {
		return fmt.Sprintf("login:loc:%s",account)
	}

	return fmt.Sprintf("login:loc:%s:%s", account, device)
}

func KeyLocations(accounts ...string) []string {
	arr := make([]string, len(accounts))
	for i, account := range accounts {
		arr[i] = KeyLocation(account, "")
	}
	return arr
}
