package core

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	inet "gx/ipfs/QmPtFaR7BWHLAjSwLh9kXcyrgTzDpuhcWLkx8ioa9RMYnx/go-libp2p-net"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
)

type StreamGeneraterImpl struct {
	node types.Node
}

func NewStreamGenerater(node types.Node) types.StreamGenerater {
	return &StreamGeneraterImpl{node}
}

func (self *StreamGeneraterImpl) GenBridge(_bid, _tid string, action byte) (inet.Stream, error) {
	ctx := context.Background()
	bid, err := peer.IDB58Decode(_bid)
	if err != nil {
		return nil, err
	}
	tid, err := peer.IDB58Decode(_tid)
	if err != nil {
		return nil, err
	}
	log4go.Info("-- [ 🌈 gen bridge ] --> (%s) --> (%s) ", _bid, _tid)
	bstream, err := self.node.Host().NewStream(ctx, bid, params.P_CHANNEL_BRIDGE)
	if err != nil {
		log4go.Error("build bridge error : %v", err)
		return nil, err
	}
	// 1 handshak bid tid
	//				   |<--------- head(9) -------->|
	//request_packet = |<- action(1) ->|<- size(8)->|<- payload ->|
	handshak := func() error {
		pp := []byte(tid.Pretty())
		size := int64(len(pp))
		pbuff := bytes.NewBuffer([]byte{action})
		binary.Write(pbuff, binary.BigEndian, size)
		head := pbuff.Bytes()
		request_packet := append(head, pp...)
		bstream.Write(request_packet)
		rbuff := make([]byte, 4)
		bstream.Read(rbuff)
		r := string(rbuff)
		log4go.Info("<-- bridge_action_handshak -- ( %s <=> %s ) : %s", bid.Pretty(), tid.Pretty(), r)
		if r == "pong" {
			return nil
		}
		return errors.New(r)
	}
	err = handshak()
	if err != nil {
		log4go.Error("bridge_handshak_error : %s", err)
		return nil, err
	}
	return bstream, nil
}

func (self *StreamGeneraterImpl) GenPingStream(targetid string) (inet.Stream, error) {
	var stream inet.Stream
	ctx := context.Background()
	findby := make(chan peer.ID)
	tid, err := peer.IDB58Decode(targetid)
	if err != nil {
		return nil, err
	}

	pi, err := self.node.FindPeer(ctx, tid, findby)
	if err != nil {
		return nil, err
	}
	// 尝试直连
	if err = self.node.Host().Connect(ctx, pi); err != nil {
		// 尝试搭桥 这里一定有返回值
		fy := <-findby
		if fy == "" {
			return nil, err
		}
		bridgeId := fy.Pretty()
		log4go.Info(" 👷‍ try_brige_service --> %s ", bridgeId)
		log4go.Info("bridge_ping_stream : %s --> %s", bridgeId, targetid)
		//test_fy := "QmYbU9PcMrNao4jVBQ4eKg9PcAbFD3Pfges4viZw762tWX" // liangc xiaomi hd
		//bridgeId = test_fy
		stream, err = self.GenBridge(bridgeId, targetid, params.BRIDGE_ACTION_PING)
	} else {
		log4go.Info(" 🌞 normal_ping_stream : --> %s", targetid)
		stream, err = self.node.Host().NewStream(ctx, tid, params.P_CHANNEL_PING)
	}
	if err != nil {
		return nil, err
	}
	return stream, nil
}

func (self *StreamGeneraterImpl) GenFileStream(filepath, targetid string, rf, rt int64) (*types.FileStreamEntity, error) {
	var stream inet.Stream
	ctx := context.Background()
	tid, err := peer.IDB58Decode(targetid)
	findby := make(chan peer.ID)
	if err != nil {
		return nil, err
	}
	// findpeer 如果能直连，则 findby 是空，否则尝试连接，失败后再尝试搭桥
	pi, err := self.node.FindPeer(ctx, tid, findby)
	if err != nil {
		return nil, err
	}
	// 1 直连
	if err = self.node.Host().Connect(ctx, pi); err != nil {
		// 2 尝试搭桥
		log4go.Info(" ❌ %v", err)
		fy := <-findby
		if fy == "" {
			return nil, err
		}
		log4go.Info("try_brige_service  🌞  👼  --> %s ", fy.Pretty())
		log4go.Info("bridge_getfile_stream : %s --> %s", fy.Pretty(), targetid)
		stream, err = self.GenBridge(fy.Pretty(), targetid, params.BRIDGE_ACTION_FILE)
	} else {
		log4go.Info("normal_getfile_stream : --> %s", targetid)
		stream, err = self.node.Host().NewStream(ctx, tid, params.P_CHANNEL_FILE)
	}

	if err != nil {
		log4go.Info("ch = %s , tid = %s", params.P_CHANNEL_FILE, tid.Pretty())
		log4go.Error("newStreamError : %s", err.Error())
		return nil, err
	}
	//				|<-------------------- head(24) ------------------->|
	// get_packet = |<- size(8) ->|<- range_from(8) ->|<- range_to(8) ->|<- filepath ->|
	req_body := []byte(filepath)
	l := int64(len(req_body))
	lenBuff := bytes.NewBuffer([]byte{})
	binary.Write(lenBuff, binary.BigEndian, l)
	binary.Write(lenBuff, binary.BigEndian, rf)
	binary.Write(lenBuff, binary.BigEndian, rt)

	req_head := lenBuff.Bytes()
	log4go.Info("head --> %d , %v", len(req_head), req_head)
	req := append(req_head, req_body...)
	_, err = stream.Write(req)
	if err != nil {
		log4go.Error("write request head error : %s", err.Error())
		return nil, err
	}
	//			|<-------------------------- head(41) ---------------------------->|
	//payload = |<- status(1) ->|<- size(8)->|<- file_md5(16) ->|<- part_md5(16) ->|<- file ->|
	buff := make([]byte, 41)
	stream.Read(buff)

	sizeBuff := buff[1:9]
	var responseSize int64

	hb := bytes.NewBuffer(sizeBuff)
	if err := binary.Read(hb, binary.BigEndian, &responseSize); err != nil {
		log4go.Error(err)
	}
	if buff[0] == byte(0) {
		log4go.Info(buff)
		defer func() {
			if stream != nil {
				stream.Close()
			}
		}()
		// decode error value
		errBuf := make([]byte, responseSize)
		stream.Read(errBuf)
		return nil, errors.New(string(errBuf))
	}
	log4go.Info("response_head (%d) -> %v", len(buff), buff)

	md5Buff := buff[9:25]
	partMd5Buff := buff[25:41]
	hash := hex.EncodeToString(md5Buff)
	parthash := hex.EncodeToString(partMd5Buff)
	return &types.FileStreamEntity{stream, responseSize, hash, parthash}, nil
}
