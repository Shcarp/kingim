package benchmark

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"kingim"
	"kingim/examples/mock"
	"kingim/logger"
	"kingim/websocket"
	"github.com/panjf2000/ants/v2"
)

const wsurl = "ws://localhost:8000"

func Test_Parallel(t *testing.T) {
	const count = 10000
	gpool, _ := ants.NewPool(50, ants.WithPreAlloc(true))
	defer gpool.Release()
	var wg sync.WaitGroup
	wg.Add(count)

	clis := make([]kingim.Client, count)
	t0 := time.Now()
	for i := 0; i < count; i++ {
		idx := i
		_ = gpool.Submit(func() {
			cli := websocket.NewClient(fmt.Sprintf("test_%v", idx), "client", websocket.ClientOptions{
				Heartbeat: kingim.DefaultHeartbeat,
			})
			// set dialer
			cli.SetDialer(&mock.WebsocketDialer{})

			// step2: 建立连接
			err := cli.Connect(wsurl)
			if err != nil {
				logger.Error(err)
			}
			clis[idx] = cli
			wg.Done()
		})
	}
	wg.Wait()
	t.Logf("logined %d cost %v", count, time.Since(t0))
	t.Logf("done connecting")
	time.Sleep(time.Second * 10)
	t.Logf("closed")

	for i := 0; i < count; i++ {
		clis[i].Close()
	}
}

func Test_Message(t *testing.T) {
	const count = 10000
	gpool, _ := ants.NewPool(50, ants.WithPreAlloc(true))
	defer gpool.Release()

	cli := websocket.NewClient(fmt.Sprintf("test_%v", 1), "client", websocket.ClientOptions{
		Heartbeat: kingim.DefaultHeartbeat,
	})
	// set dialer
	cli.SetDialer(&mock.WebsocketDialer{})

	// step2: 建立连接
	err := cli.Connect(wsurl)
	if err != nil {
		logger.Error(err)
	}
	msg := []byte(strings.Repeat("hello", 1000))
	t0 := time.Now()
	for i := 0; i < count; i++ {
		_ = gpool.Submit(func() {
			_ = cli.Send(msg)
		})
	}
	recv := 0
	for {
		frame, err := cli.Read()
		if err != nil {
			logger.Info(err)
			break
		}
		if frame.GetOpCode() != kingim.OpBinary {
			continue
		}
		recv++
		if recv == count { // 接收完消息
			break
		}
	}

	t.Logf("message %d cost %v", count, time.Since(t0))
}
