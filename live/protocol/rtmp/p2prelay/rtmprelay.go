package p2prelay

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/live/protocol/amf"
	"github.com/SmartMeshFoundation/Perception/live/protocol/rtmp/core"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"io"
	"log"
)

var (
	STOP_CTRL = "RTMPRELAY_STOP"
)

type RtmpRelay struct {
	pull                 peer.ID
	push                 string
	cs_chan              chan core.ChunkStream
	sndctrl_chan         chan string
	connectPlayClient    *core.P2PConnClient
	connectPublishClient *core.ConnClient
	startflag            bool
	node                 types.Node
}

func NewRtmpRelay(node types.Node, pull peer.ID, pushurl string) *RtmpRelay {
	return &RtmpRelay{
		pull:                 pull,
		push:                 pushurl,
		cs_chan:              make(chan core.ChunkStream, 500),
		sndctrl_chan:         make(chan string),
		connectPlayClient:    nil,
		connectPublishClient: nil,
		startflag:            false,
		node:                 node,
	}
}

func (self *RtmpRelay) rcvPlayChunkStream() {
	log.Println("rcvPlayRtmpMediaPacket connectClient.Read...")
	for {
		var rc core.ChunkStream

		if self.startflag == false {
			self.connectPlayClient.Close(nil)
			log.Printf("rcvPlayChunkStream close: pull=%s, push=%s", self.pull, self.push)
			break
		}
		err := self.connectPlayClient.Read(&rc)

		if err != nil && err == io.EOF {
			break
		}
		//log.Printf("connectPlayClient.Read return rc.TypeID=%v length=%d, err=%v", rc.TypeID, len(rc.Data), err)
		switch rc.TypeID {
		case 20, 17:
			r := bytes.NewReader(rc.Data)
			vs, err := self.connectPlayClient.DecodeBatch(r, amf.AMF0)

			log.Printf("rcvPlayRtmpMediaPacket: vs=%v, err=%v", vs, err)
		case 18:
			log.Printf("rcvPlayRtmpMediaPacket: metadata....")
		case 8, 9:
			self.cs_chan <- rc
		}
	}
}

func (self *RtmpRelay) sendPublishChunkStream() {
	for {
		select {
		case rc := <-self.cs_chan:
			//log.Printf("sendPublishChunkStream: rc.TypeID=%v length=%d", rc.TypeID, len(rc.Data))
			self.connectPublishClient.Write(rc)
		case ctrlcmd := <-self.sndctrl_chan:
			if ctrlcmd == STOP_CTRL {
				self.connectPublishClient.Close(nil)
				log.Printf("sendPublishChunkStream close: pull=%s, push=%s", self.pull, self.push)
				break
			}
		}
	}
}

func (self *RtmpRelay) Start() error {
	if self.startflag {
		err := errors.New(fmt.Sprintf("The rtmprelay already started, pull=%s, push=%s", self.pull, self.push))
		return err
	}

	self.connectPlayClient = core.NewP2PConnNode(self.node)
	self.connectPublishClient = core.NewConnNode(self.node)

	log.Printf("play server addr:%v starting....", self.pull)
	err := self.connectPlayClient.Start(self.pull, "play")
	if err != nil {
		log.Printf("connectPlayClient.Start url=%v error", self.pull)
		return err
	}

	log.Printf("publish server addr:%v starting....", self.push)
	err = self.connectPublishClient.Start(self.push, "publish")
	if err != nil {
		log.Printf("connectPublishClient.Start url=%v error", self.push)
		self.connectPlayClient.Close(nil)
		return err
	}

	self.startflag = true
	go self.rcvPlayChunkStream()
	go self.sendPublishChunkStream()

	return nil
}

func (self *RtmpRelay) Stop() {
	if !self.startflag {
		log.Printf("The rtmprelay already stoped, playurl=%s, publishurl=%s", self.pull, self.push)
		return
	}

	self.startflag = false
	self.sndctrl_chan <- STOP_CTRL

}
