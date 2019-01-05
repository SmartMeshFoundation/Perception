package live

import (
	"errors"
	"fmt"
	"github.com/alecthomas/log4go"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/live/protocol/rtmp"
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
	ls.controls = NewControls(ls.rtmpStream, ls.rtmpAddr)
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
	var rtmpServer *rtmp.Server
	rtmpServer = rtmp.NewRtmpServer(self.rtmpStream)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log4go.Error("RTMP server panic: %v", r)
			}
		}()
		log4go.Info("RTMP Listen On %s , push path : /live/%s", self.rtmpAddr, self.node.Host().ID().Pretty())
		rtmpServer.Serve(rtmpListen)
	}()
	return nil
}
