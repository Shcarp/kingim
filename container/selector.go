package container

import (
	"kingim"
	"kingim/wire/pkt"
	"hash/crc32"
)

// 生成一个has值， 通过hash 值来选择服务
func HashCode(key string) int {
	hash32 := crc32.NewIEEE()
	hash32.Write([]byte(key))
	return int(hash32.Sum32())
}
// 用来选择服务
type Selector interface {
	Lookup(*pkt.Header, []kingim.Service) string
}

