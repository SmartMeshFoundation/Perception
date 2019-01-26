package live

import (
	"errors"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/live/protocol/rtmp"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"net"
)

type LiveServerImpl struct {
	rtmpAddr   string // :11935
	controls   *Controls
	rtmpStream *rtmp.RtmpStream
	node       types.Node
}

func NewLiveServer(node types.Node, port int) types.LiveServer {
	ls := &LiveServerImpl{
		node:       node,
		rtmpAddr:   fmt.Sprintf(":%d", port),
		rtmpStream: rtmp.NewRtmpStream(),
	}
	ls.controls = NewControls(node, ls.rtmpStream, ls.rtmpAddr)
	return ls
}

func (self *LiveServerImpl) GetControlHandler(pattern string) (types.HttpHandlerFn, error) {
	if fn := self.controls.Handler(pattern); fn == nil {
		return nil, errors.New("live.control handler pattern error : " + pattern)
	} else {
		return fn, nil
	}
}

func (self *LiveServerImpl) Stop() error {
	return nil
}

func (self *LiveServerImpl) Start() error {
	rtmpListen, err := net.Listen("tcp", self.rtmpAddr)
	if err != nil {
		log4go.Error(err)
		return err
	}
	go func() {
		select {
		case <-self.node.Context().Done():
			rtmpListen.Close()
		}
	}()
	var rtmpServer *rtmp.Server
	rtmpServer = rtmp.NewRtmpServer(self.node, self.rtmpStream)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log4go.Error("RTMP server panic: %v", r)
			}
		}()
		log4go.Info("--------------------------------------------------------------------------")
		log4go.Info("rtmp on %s, /cc14514/%s", self.rtmpAddr, self.node.Host().ID().Pretty())
		log4go.Info("--------------------------------------------------------------------------")
		rtmpServer.Serve(rtmpListen)
	}()
	return nil
}
