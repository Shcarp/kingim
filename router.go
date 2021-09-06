package kingim

import (
	"fmt"
	"kingim/wire/pkt"
	"sync"
)

type Router struct {
	handers *FuncTree
	pool sync.Pool   // 对象池，提高性能
}

type FuncTree struct {
	node map[string]HandlersChain
}

func NewTree() *FuncTree {
	return &FuncTree{node: make(map[string]HandlersChain, 10)}
}

func NewRouter() *Router {
	r := &Router{
		handers: NewTree(),
	}
	r.pool.New = func() interface{} {
		return BuildContext()     // 创建上下文对象池
	}
	return r
}
/**
首先从pool中取出一个ContextImpl 对象，然后注入
请求包： pkt.LogicPkt
信息分发器 Dispather
会话管理器 SessionStorage
发送方会话 Session
 */

func (s*Router) Serve(packet *pkt.LogicPkt ,dispather Dispather, cache SessionStorage, session Session) error {
	if dispather == nil {
		return fmt.Errorf("dispather is nil")
	}
	if cache == nil {
		return fmt.Errorf("cache is nil ")
	}
	ctx := s.pool.Get().(*ContextImpl)
	ctx.reset()
	ctx.request = packet
	ctx.SessionStorage = cache
	ctx.session = session
	ctx.Dispather = dispather
	s.serveContext(ctx)
	s.pool.Put(ctx)
	return nil
}

// 链路处理
func (s*Router) serveContext(ctx *ContextImpl) {
	chain, ok := s.handers.Get(ctx.Header().Command)
	if !ok {
		ctx.handlers = []HandlerFunc{handleNoFound}
		ctx.Next()
		return
	}
	ctx.handlers =chain
	ctx.Next()   //责任链
}

func (s*Router) Handle(command string, handlers ...HandlerFunc) {
	s.handers.Add(command,handlers)
}

func (t*FuncTree) Add(path string ,handles []HandlerFunc) {
	t.node[path] = append(t.node[path], handles...)
}

func handleNoFound(ctx Context) {
	_ = ctx.Resp(pkt.Status_NotImplemented, &pkt.ErrorResp{Message: "NotImplemented"})
}

func (t*FuncTree) Get(path string) (HandlersChain,bool) {
	f,ok := t.node[path]
	return f,ok
}