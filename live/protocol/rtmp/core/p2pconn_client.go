package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/cmd/utils"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/live/av"
	"github.com/SmartMeshFoundation/Perception/live/protocol/amf"
	"github.com/SmartMeshFoundation/Perception/params"
	inet "gx/ipfs/QmPtFaR7BWHLAjSwLh9kXcyrgTzDpuhcWLkx8ioa9RMYnx/go-libp2p-net"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"io"
	"log"
)

type P2PConnClient struct {
	done       bool
	transID    int
	peer       string
	tcurl      string
	app        string
	title      string
	query      string
	curcmdName string
	streamid   uint32
	p2pconn    *P2PConn
	encoder    *amf.Encoder
	decoder    *amf.Decoder
	bytesw     *bytes.Buffer
	node       types.Node
}

func NewP2PConnNode(node types.Node) *P2PConnClient {
	cn := NewP2PConnClient()
	cn.node = node
	return cn
}

func NewP2PConnClient() *P2PConnClient {
	return &P2PConnClient{
		transID: 1,
		bytesw:  bytes.NewBuffer(nil),
		encoder: &amf.Encoder{},
		decoder: &amf.Decoder{},
	}
}

func (self *P2PConnClient) DecodeBatch(r io.Reader, ver amf.Version) (ret []interface{}, err error) {
	vs, err := self.decoder.DecodeBatch(r, ver)
	return vs, err
}

func (self *P2PConnClient) readRespMsg() error {
	var err error
	var rc ChunkStream
	for {
		if err = self.p2pconn.Read(&rc); err != nil {
			return err
		}
		if err != nil && err != io.EOF {
			return err
		}
		switch rc.TypeID {
		case 20, 17:
			r := bytes.NewReader(rc.Data)
			vs, _ := self.decoder.DecodeBatch(r, amf.AMF0)

			log.Printf("readRespMsg: vs=%v", vs)
			for k, v := range vs {
				switch v.(type) {
				case string:
					switch self.curcmdName {
					case cmdConnect, cmdCreateStream:
						if v.(string) != respResult {
							return errors.New(v.(string))
						}

					case cmdPublish:
						if v.(string) != onStatus {
							return ErrFail
						}
					}
				case float64:
					switch self.curcmdName {
					case cmdConnect, cmdCreateStream:
						id := int(v.(float64))

						if k == 1 {
							if id != self.transID {
								return ErrFail
							}
						} else if k == 3 {
							self.streamid = uint32(id)
						}
					case cmdPublish:
						if int(v.(float64)) != 0 {
							return ErrFail
						}
					}
				case amf.Object:
					objmap := v.(amf.Object)
					switch self.curcmdName {
					case cmdConnect:
						code, ok := objmap["code"]
						if ok && code.(string) != connectSuccess {
							return ErrFail
						}
					case cmdPublish:
						code, ok := objmap["code"]
						if ok && code.(string) != publishStart {
							return ErrFail
						}
					}
				}
			}

			return nil
		}
	}
}

func (self *P2PConnClient) writeMsg(args ...interface{}) error {
	self.bytesw.Reset()
	for _, v := range args {
		if _, err := self.encoder.Encode(self.bytesw, v, amf.AMF0); err != nil {
			return err
		}
	}
	msg := self.bytesw.Bytes()
	c := ChunkStream{
		Format:    0,
		CSID:      3,
		Timestamp: 0,
		TypeID:    20,
		StreamID:  self.streamid,
		Length:    uint32(len(msg)),
		Data:      msg,
	}
	self.p2pconn.Write(&c)
	return self.p2pconn.Flush()
}

func (self *P2PConnClient) writeConnectMsg() error {
	event := make(amf.Object)
	event["app"] = self.app
	event["type"] = "nonprivate"
	event["flashVer"] = "FMS.3.1"
	event["tcUrl"] = self.tcurl
	self.curcmdName = cmdConnect

	log.Printf("writeConnectMsg: self.transID=%d, event=%v", self.transID, event)
	if err := self.writeMsg(cmdConnect, self.transID, event); err != nil {
		return err
	}
	return self.readRespMsg()
}

func (self *P2PConnClient) writeCreateStreamMsg() error {
	self.transID++
	self.curcmdName = cmdCreateStream

	log.Printf("writeCreateStreamMsg: self.transID=%d", self.transID)
	if err := self.writeMsg(cmdCreateStream, self.transID, nil); err != nil {
		return err
	}

	for {
		err := self.readRespMsg()
		if err == nil {
			return err
		}

		if err == ErrFail {
			log4go.Error(err)
			return err
		}
	}

}

func (self *P2PConnClient) writePublishMsg() error {
	self.transID++
	self.curcmdName = cmdPublish
	if err := self.writeMsg(cmdPublish, self.transID, nil, self.title, publishLive); err != nil {
		return err
	}
	return self.readRespMsg()
}

func (self *P2PConnClient) writePlayMsg() error {
	self.transID++
	self.curcmdName = cmdPlay
	log.Printf("writePlayMsg: self.transID=%d, cmdPlay=%v, self.title=%v", self.transID, cmdPlay, self.title)
	if err := self.writeMsg(cmdPlay, 0, nil, self.title); err != nil {
		return err
	}
	return self.readRespMsg()
}

func (self *P2PConnClient) Start(target peer.ID, method string) error {


	self.peer = target.Pretty()
	self.app = "cc14514"
	self.title = target.Pretty()
	self.query = ""
	self.tcurl = fmt.Sprintf("rtmp://127.0.0.1:%d/%s", params.LivePort, self.app)

	var (
		tc     inet.Stream
		ctx    = context.Background()
		findby = make(chan peer.ID)
		p      = params.P_CHANNEL_LIVE
	)
	pi, err := self.node.FindPeer(ctx, target, findby)
	if err != nil {
		return err
	}
	if err = self.node.Host().Connect(ctx, pi); err != nil {
		// 尝试搭桥 这里一定有返回值
		fy := <-findby
		if fy == "" {
			return err
		}
		bridgeId := fy.Pretty()
		log4go.Info(" 👷‍ try_agent_brige_service : %s --> %s ", bridgeId, target)
		tc, err = utils.Astab.GenBridge(ctx, fy, target, p)
	} else {
		log4go.Info(" 🌞 normal_%s_stream : --> %s", p, target)
		tc, err = self.node.Host().NewStream(ctx, target, p)
	}

	self.p2pconn = NewP2PConn(tc, 4*1024)

	log.Println("HandshakeClient....")
	if err := self.p2pconn.HandshakeClient(); err != nil {
		return err
	}

	log.Println("writeConnectMsg....")
	if err := self.writeConnectMsg(); err != nil {
		return err
	}
	log.Println("writeCreateStreamMsg....")
	if err := self.writeCreateStreamMsg(); err != nil {
		log.Println("writeCreateStreamMsg error", err)
		return err
	}

	log.Println("method control:", method, av.PUBLISH, av.PLAY)
	if method == av.PUBLISH {
		if err := self.writePublishMsg(); err != nil {
			return err
		}
	} else if method == av.PLAY {
		if err := self.writePlayMsg(); err != nil {
			return err
		}
	}

	return nil
}

func (self *P2PConnClient) Write(c ChunkStream) error {
	if c.TypeID == av.TAG_SCRIPTDATAAMF0 ||
		c.TypeID == av.TAG_SCRIPTDATAAMF3 {
		var err error
		if c.Data, err = amf.MetaDataReform(c.Data, amf.ADD); err != nil {
			return err
		}
		c.Length = uint32(len(c.Data))
	}
	return self.p2pconn.Write(&c)
}

func (self *P2PConnClient) Flush() error {
	return self.p2pconn.Flush()
}

func (self *P2PConnClient) Read(c *ChunkStream) (err error) {
	return self.p2pconn.Read(c)
}

func (self *P2PConnClient) GetInfo() (app string, name string, peer string) {
	app = self.app
	name = self.title
	peer = self.peer
	return
}

func (self *P2PConnClient) GetStreamId() uint32 {
	return self.streamid
}

func (self *P2PConnClient) Close(err error) {
	self.p2pconn.Close()
}
