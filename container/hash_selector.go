package container

import (
	"kingim"
	"kingim/wire/pkt"
)

type HashSelector struct {

}

// 选择合适的服务
func (s*HashSelector) Lookup(header *pkt.Header, srvs []kingim.Service) string {
	ll:= len(srvs)
	code := HashCode(header.ChannelId)
	return srvs[code % ll].ServiceID()
}
