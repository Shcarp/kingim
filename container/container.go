package container

import (
	"bytes"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"kingim"
	"kingim/logger"
	"kingim/naming"
	"kingim/tcp"
	"kingim/wire"
	"kingim/wire/pkt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	stateUninitialized  = iota    // 转态为初始化
	stateInitialized			// 初始化
	stateStarted				// 开始
	stateClosed                 // 关闭
)

const (
	StateYoung = "young"
	StateAdult = "adult"
)
const (
	KeyServiceState = "service_state"
)

type Container struct {
	sync.RWMutex  // 读写锁 读的时候不影响写，写的时候会组织所有锁
	Naming naming.Naming
	Srv kingim.Server
	state uint32
	srvclients map[string]ClientMap
	selector Selector
	dialer kingim.Dialer
	deps map[string]struct{}   // 依赖的服务
}

var log = logger.WithField("module","container")

var c = &Container{
	state: 0,
	selector: &HashSelector{},
	deps : make(map[string]struct{}),
}

func Default() *Container {
	return c
}

func Init(srv kingim.Server, deps ...string) error {
	if !atomic.CompareAndSwapUint32(&c.state, stateUninitialized, stateInitialized) {
		return errors.New("has Initialized")
	}
	c.Srv = srv   // 将kingim.Server 注入容器
	for _,dep := range deps {
		if _,ok := c.deps[dep]; ok {  // 如果已存在则跳过， 不存在则添加一个空结构体
			continue
		}
		c.deps[dep] = struct{}{}
	}
	log.WithField("func", "Init").Infof("srv %s:%s - deps %v", srv.ServiceID(), srv.ServiceName(), c.deps)
	c.srvclients = make(map[string]ClientMap, len(deps))
	return nil
}

func SerDialer(dialer kingim.Dialer) {
	c.dialer = dialer
}

func EnableMonitor(listen string) error {
	return nil
}
func SetSelector(selector Selector) {
	c.selector = selector
}
func SetServiceNaming(nm naming.Naming) {
 	c.Naming = nm
}


func Start()error {
	if c.Naming == nil {
		return fmt.Errorf("naming is nil")
	}
	if !atomic.CompareAndSwapUint32(&c.state, stateInitialized, stateStarted ) {
		return errors.New("has started")
	}
	// 启动server 服务
	go func(srv kingim.Server) {
		err := srv.Start()
		if err!= nil {
			log.Errorln(err)
		}
	}(c.Srv)
	// 与依赖的服务建立连接
	for service := range c.deps {
		go func(service string) {
			err := connectToService(service)
			if err != nil {
				log.Errorln(err)
			}
		}(service)
	}
	// 服务注册
	if c.Srv.PublicAddress() != "" && c.Srv.PublicPort() != 0 {
		err := c.Naming.Register(c.Srv)
		if err != nil {
			log.Errorln(err)
		}
	}
	// 等待退出信号
	c := make (chan os.Signal, 1)
	signal.Notify(c,syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	log.Infoln("shutdown", <-c)
	return shutdown()
}

// 下游的服务与依赖的服务建立的连接被包装为client 比如网关与登录服务建立的连接 所以有多个clients每个维护一张clientsMap表示
// 一个容器包含一个服务，clients 是该服务与其关联的服务的连接，一个服务可能与多个服务关联
func connectToService(serviceName string) error {
	clients := NewClients(10)
	c.srvclients[serviceName] = clients
	delay := time.Second *10
	err := c.Naming.Subscribe(serviceName, func(services []kingim.ServiceRegistration) { // 一个服务可能有多个server提供
		for _, service := range services {
			if _,ok := clients.Get(service.ServiceID()); ok {   // 如果已经注册则跳过
				continue
			}
			log.WithField("func", "connectToService").Infof("watch a new service", service)
			service.GetMeta()[KeyServiceState] = StateYoung
			go func() {
				time.Sleep(delay)
				service.GetMeta()[KeyServiceState] = StateAdult
			}()
			_,err := buildClient(clients, service)
			if err != nil {
				logger.Warn(err)
			}
		}
	})
	if err != nil {
		return err
	}
	services, err := c.Naming.Find(serviceName)
	if err != nil {
		return err
	}
	log.Info("find service", services)
	for _,service := range services {
		service.GetMeta()[KeyServiceState] = StateAdult
		_, err := buildClient(clients, service)
		if err != nil {
			logger.Warn(err)
		}
	}
	return nil
}

// 建立连接
func buildClient(clients ClientMap, service kingim.ServiceRegistration ) (kingim.Client, error) {
	c.Lock()
	defer c.Unlock()
	var (
		id = service.ServiceID()
		name = service.ServiceName()
		meta = service.GetMeta()
	)
	if _, ok := clients.Get(id); ok {
		return nil,nil
	}
	if service.GetProtocol() != string(wire.ProtocolTCP) {
		return nil,fmt.Errorf("unexpected service Protocol: %s", service.GetProtocol())
	}
	cli := tcp.NewClientWithProps(id, name, meta, tcp.ClientOptions{
		HeartBeat: kingim.DefaultHeartbeat,
		ReadWait: kingim.DefaultReadWait,
		WriteWait: kingim.DefaultWriteWait,
	})
	if c.dialer == nil {
		return nil, fmt.Errorf("dialer is nil")
	}
	cli.SetDialer(c.dialer)
	err := cli.Connect(service.DialURL())
	if err != nil {
		return nil,err
	}
	go func(cli kingim.Client) {
		err := readloop(cli)
		if err != nil {
			log.Debug(err)
		}
		clients.Remove(id)
		cli.Close()
	}(cli)
	clients.Add(cli)
	return cli, nil
}

func shutdown() error {
	if !atomic.CompareAndSwapUint32(&c.state, stateStarted, stateClosed) {
		return errors.New("has closed")
	}
	ctx, cancel := context.WithTimeout(context.TODO(),time.Second*10)
	defer cancel()
	err := c.Srv.Shutdown(ctx)    // 关闭服务器
	if err != nil {
		log.Error(err)
	}
	err = c.Naming.Deregister(c.Srv.ServiceID())
	if err != nil {
		return err
	}
	for dep := range c.deps {
		_ = c.Naming.Unsubscribe(dep)
	}
	log.Infoln("shutdown")
	return nil
}
//下行建立客户端之后开启一个协程，循环读取传，如果读取到信息且合法，使用pushMessage将信息发送到需要发送的channel

func readloop (cli kingim.Client) error {
	log := logger.WithFields(logger.Fields{
		"module": "container",
		"func" : "readLoop",
	})
	log.Infoln("readloop stated of %s %s",cli.ServiceID(),cli.ServiceName())
	for  {
		frame, err := cli.Read()
		if err != nil {
			return err
		}
		if frame.GetOpCode() == kingim.OpBinary {
			continue
		}
		buf := bytes.NewBuffer(frame.GetPayload())
		packet,err := pkt.MustReadLogicPkt(buf)
		if err != nil {
			log.Info(err)
			continue
		}
		err = pushMessage(packet)
		if err != nil {
			log.Info(err)
		}
	}
}

func pushMessage(packet *pkt.LogicPkt) error {
	server, _  := packet.GetMeta(wire.MetaDestServer) // 信息抵达的服务
	if server != c.Srv.ServiceID() {
		return fmt.Errorf("dest_server is incorrect, %s != %s", server,c.Srv.ServiceID())
	}
	channels, ok := packet.GetMeta(wire.MetaDestChannels) // 信息接收方
	if !ok {
		return fmt.Errorf("dest_channels is nil")
	}
	channelIds := strings.Split(channels.(string), ",")
	packet.DelMeta(wire.MetaDestServer)
	packet.DelMeta(wire.MetaDestChannels)
	payload := pkt.Marshal(packet)
	log.Debugf("push to %v %v", channels, packet)
	for _, channel := range channelIds {
		err := c.Srv.Push(channel, payload)   // s.Srv.Push  ch.Push  writeloop  writeFrame
		if err != nil {
			log.Debug(err)
		}
	}
	return nil
}

func Push(server string, p *pkt.LogicPkt) error {
	p.AddStringMeta(wire.MetaDestServer, server) // 将要送达网关的serverName   // 在信息中添加server(服务网关)
	return c.Srv.Push(server, pkt.Marshal(p))
}

// 上行，根据服务的名称在创建的服务列别中寻找一个合适的服务，通过这个服务将信息发送出去，

func Forward(serviceName string, packet *pkt.LogicPkt) error {
	if packet == nil {
		return errors.New("packet is nil")
	}
	if packet.Command == "" {
		return errors.New("command is empty in packet")
	}
	if packet.ChannelId == "" {
		return errors.New("channelId is nil")
	}
	return ForwardWithSelector(serviceName, packet, c.selector)  // 选择服务
}

func ForwardWithSelector(serviceName string, packet *pkt.LogicPkt, selector Selector) error {
	cli, err := lookup(serviceName, &packet.Header, selector)
	if err != nil {
		return err
	}
	packet.AddStringMeta(wire.MetaDestServer, c.Srv.ServiceID())
	log.Debugf("forward message to %v with %s", cli.ServiceID(), &packet.Header)
	return cli.Send(pkt.Marshal(packet))
}
func lookup(serviceName string, header *pkt.Header,  selector Selector ) (kingim.Client, error) {
	clients, ok := c.srvclients[serviceName]
	if !ok {
		return nil,fmt.Errorf("service %s not found", serviceName)
	}
	srvs := clients.Service(KeyServiceState, StateAdult)   // 转态为SateAdult 的服务才会被使用
	if len(srvs) == 0 {
		return nil,fmt.Errorf("no services found for %s",serviceName)
	}
	id := selector.Lookup(header, srvs)    // 选择一个合适的服务
	if cli,ok := clients.Get(id); ok {
		return cli, nil
	}   // 判断选择的服务在列表中是否存在
	return nil,fmt.Errorf("no client found")
}

