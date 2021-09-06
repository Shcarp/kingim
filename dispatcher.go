package kingim

import "kingim/wire/pkt"

type Dispather interface {
	Push(gateway string, channels []string, p *pkt.LogicPkt) error    // 在Dispatcher 在Router中被创建时注入进来   // 传入的是真正使用的服务的push方法
}
