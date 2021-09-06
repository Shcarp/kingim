package handler

import (
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"kingim"
	"kingim/wire"
	"kingim/wire/pkt"
	"testing"
)

func TestLoginHandler_DoSysLogin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dispather := kingim.NewMockDispather(ctrl)
	cache := kingim.NewMockSessionStorage(ctrl)
	session := &pkt.Session{
		ChannelId: "channel1",
		Account:   "test1",
		GateId:    "gateway1",
	}
	// resp
	dispather.EXPECT().Push(session.GateId, []string{"channel1"}, gomock.Any()).Times(1)
	// kickout notify
	dispather.EXPECT().Push(session.GateId, []string{"channel2"}, gomock.Any()).Times(1)

	cache.EXPECT().GetLocation(session.Account, "").DoAndReturn(func(account string, device string) (*kingim.Location, error) {
		return &kingim.Location{
			ChannelId: "channel2",
			GateId:    "gateway1",
		}, kingim.ErrSessionNil
	})

	cache.EXPECT().Add(gomock.Any()).Times(1).DoAndReturn(func(add *pkt.Session) error {
		assert.Equal(t, session.ChannelId, add.ChannelId)
		assert.Equal(t, session.Account, add.Account)
		return nil
	})

	loginreq := pkt.New(wire.CommandLoginSignIn).WriteBody(session)

	r := kingim.NewRouter()
	// login
	loginHandler := NewLoginHandler()
	r.Handle(wire.CommandLoginSignIn, loginHandler.DoSysLogin)

	err := r.Serve(loginreq, dispather, cache, session)
	assert.Nil(t, err)
}