package core

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/agents"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	inet "gx/ipfs/QmPtFaR7BWHLAjSwLh9kXcyrgTzDpuhcWLkx8ioa9RMYnx/go-libp2p-net"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	"os"
	"path"
	"sync"
	"time"
	//"gx/ipfs/QmTKsRYeY4simJyf37K93juSq75Lo8MVCDJ7owjmf46u8W/go-context/io"
	"github.com/SmartMeshFoundation/Perception/agents/pb"
	"github.com/SmartMeshFoundation/Perception/cmd/utils"
	ggio "gx/ipfs/QmdxUuburamoF6zF9qjeQC4WYcWGbWuRmdLacMEsW8ioD8/gogo-protobuf/io"
	"io"
	//ggio "github.com/gogo/protobuf/io"
)

var (
	PING, PANG, PONG          = "ping", "pang", "pong"
	HANDSHAK_ERR, HANDSHAK_OK = PANG, PONG
)

type bufferedWriteCloser interface {
	ggio.WriteCloser
	Flush() error
}

type bufferedDelimitedWriter struct {
	*bufio.Writer
	ggio.WriteCloser
}

func newBufferedDelimitedWriter(str io.Writer) bufferedWriteCloser {
	w := bufio.NewWriter(str)
	return &bufferedDelimitedWriter{
		Writer:      w,
		WriteCloser: ggio.NewDelimitedWriter(w),
	}
}

type StreamHandler struct {
	node types.Node
}

func NewStreamHandler(node types.Node) *StreamHandler {
	return &StreamHandler{node}
}

// TODO 完成 agents 的桥接
// handler "/channel/agents" stream
func (self *StreamHandler) agentsHandler(s inet.Stream) {
	ctx := context.Background()
	// remote peer
	mPeer := s.Conn().RemotePeer()
	r := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
	w := newBufferedDelimitedWriter(s)
	for {
		// receive msg
		pmes := new(agents_pb.AgentMessage)
		err := r.ReadMsg(pmes)
		switch err {
		case io.EOF:
			s.Close()
			return
		case nil:
		default:
			s.Reset()
			log4go.Error("Error unmarshaling data: %s", err)
			return
		}

		// TODO 特别照顾 桥接的 请求，如果是桥接的请求则不再返回了,全部在内部完成
		if agents_pb.AgentMessage_BRIDGE == pmes.GetType() {
			err = utils.Astab.Bridge(ctx, s, pmes, func(resp *agents_pb.AgentMessage) error {
				// send out response msg
				err = w.WriteMsg(resp)
				if err == nil {
					err = w.Flush()
				}
				if err != nil {
					s.Reset()
					log4go.Error("send response error: %s", err)
					return err
				}
				return nil
			})
			if err != nil {
				log4go.Error("Error agent_bridge : %s", err)
			}
			if s != nil {
				s.Close()
			}
			log4go.Info("💡 agent_bridge_done.")
			return
		}
		// get handler for this msg type.
		handler := utils.Astab.HandlerForMsgType(pmes.GetType())
		if handler == nil {
			s.Reset()
			log4go.Error("got back nil handler from handlerForMsgType")
			return
		}

		// dispatch handler.
		rpmes, err := handler(ctx, mPeer, pmes)
		if err != nil {
			s.Reset()
			log4go.Error("handle message error: %s", err)
			return
		}

		// if nil response, return it before serializing
		if rpmes == nil {
			log4go.Error("got back nil response from request")
			continue
		}

		// send out response msg
		err = w.WriteMsg(rpmes)
		//log4go.Info(" <- out : peer = %s , type = %v , err = %v", mPeer.Pretty(), rpmes.Type, err)

		if err == nil {
			err = w.Flush()
		}
		if err != nil {
			s.Reset()
			log4go.Error("send response error: %s", err)
			return
		}
	}

}

func (self *StreamHandler) agentsRestHandler(s inet.Stream) {
	self.node.GetAgentServer().RestAgent(s, agents.RestAgentConfig)
}

func (self *StreamHandler) agentsIpfsapiHandler(s inet.Stream) {
	self.node.GetAgentServer().IpfsAgent(s, agents.IpfsApiAgentConfig)
}
func (self *StreamHandler) agentsIpfsgatewayHandler(s inet.Stream) {
	self.node.GetAgentServer().IpfsAgent(s, agents.IpfsGatewayAgentConfig)
}
func (self *StreamHandler) agentsWeb3wsHandler(s inet.Stream) {
	self.node.GetAgentServer().Web3Agent(s, agents.Web3WsAgentConfig)
}
func (self *StreamHandler) agentsWeb3rpcHandler(s inet.Stream) {
	self.node.GetAgentServer().Web3Agent(s, agents.Web3RpcAgentConfig)
}

func (self *StreamHandler) bridgeHandler(s inet.Stream) {
	var (
		ts       inet.Stream
		wg       sync.WaitGroup
		BUF_SIZE = 1024
		timeout  = time.Second * 3
	)
	s.SetDeadline(time.Now().Add(timeout))
	defer func() {
		log4go.Warn(" 🌉 bridgeHandler bye bye . ")
		if s != nil {
			s.Close()
		}
		if ts != nil {
			ts.Close()
		}
	}()
	// ----------------------------------------------------------
	// 1 handshak bid tid
	//				   |<--------- head(9) -------->|
	//request_packet = |<- action(1) ->|<- size(8)->|<- payload ->|
	// ----------------------------------------------------------
	ts, e := self.bridge_handshak(s)
	ts.SetDeadline(time.Now().Add(timeout))
	if e != nil {
		s.Write([]byte(HANDSHAK_ERR))
		return
	} else {
		s.Write([]byte(HANDSHAK_OK))
	}
	// ----------------------------------------------------------
	// 2 gen bridge channel
	// ----------------------------------------------------------
	wg.Add(2)
	go func() {
		defer wg.Done()
		buf := make([]byte, BUF_SIZE)
		for {
			i, e := s.Read(buf)
			if e != nil {
				log4go.Error("g1_s_read_err ==> %v", e)
				if ts != nil {
					ts.Close()
				}
				return
			}
			_, e = ts.Write(buf[0:i])
			if e != nil {
				log4go.Error("g1_ts_write_err ==> %v", e)
				if s != nil {
					s.Close()
				}
				return
			}
		}
		log4go.Warn(" ⚠️  ⚠️  g1 bye bye .")
	}()
	go func() {
		defer wg.Done()
		buf := make([]byte, BUF_SIZE)
		for {
			i, e := ts.Read(buf)
			if e != nil {
				log4go.Error("g2_ts_read_err ==> %v", e)
				if s != nil {
					s.Close()
				}
				return
			}
			_, e = s.Write(buf[0:i])
			if e != nil {
				log4go.Error("g2_s_write_err ==> %v", e)
				if ts != nil {
					ts.Close()
				}
				return
			}
		}
		log4go.Warn(" ⚠️  ⚠️  g2 bye bye .")
	}()

	wg.Wait()
}

func (self *StreamHandler) bridge_handshak(s inet.Stream) (inet.Stream, error) {
	var (
		ctx    = context.Background()
		size   int64
		buf    = make([]byte, 9)
		action byte
		pc     protocol.ID
	)
	i, e := s.Read(buf)
	if e != nil {
		log4go.Error(e)
		return nil, e
	} else if i != 9 {
		log4go.Error("error : %d , head = %v", i, buf)
		return nil, errors.New(fmt.Sprintf("%d bad length of bridge head", i))
	}
	action = buf[0]
	if !isBridgeAction(action) {
		log4go.Error("error : %d , %s", i, action)
		return nil, errors.New(fmt.Sprintf("%d bad action of bridge service", action))
	}

	// get payload size
	hb := bytes.NewBuffer(buf[1:])
	if err := binary.Read(hb, binary.BigEndian, &size); err != nil {
		log4go.Error(err)
		return nil, err
	}
	// read payload
	buf = make([]byte, size)
	i, e = s.Read(buf)
	if e != nil {
		log4go.Error(e)
		return nil, e
	} else if int64(i) != size {
		log4go.Error("error : %d , %s", i, string(buf))
		return nil, errors.New(fmt.Sprintf("%d bad length of payload size", i))
	}

	// build tid from payload
	tid, e := peer.IDB58Decode(string(buf))
	if e != nil {
		log4go.Error(e)
		return nil, e
	}

	// ping tid
	ts, e := self.node.Host().NewStream(ctx, tid, params.P_CHANNEL_PING)
	if e != nil {
		fmt.Println("ping_error :", e)
		return nil, e
	}
	defer func() {
		if ts != nil {
			ts.Close()
		}
	}()
	i, e = ts.Write([]byte(PING))
	if e != nil {
		log4go.Error("ping_error stream is nil")
		return nil, e
	}

	// ping response
	log4go.Info("wait feedback. %d", i)
	res := make([]byte, 4)
	if i, e := ts.Read(res); e != nil {
		log4go.Error(e)
		return nil, e
	} else if string(res) != HANDSHAK_OK {
		log4go.Info("(%d) remote recv : %s", i, string(res))
		log4go.Error(e)
		return nil, errors.New(fmt.Sprintf("bad awsner from tid : %s", string(res)))
	}

	//if pong handshak over
	switch action {
	case params.BRIDGE_ACTION_PING:
		pc = params.P_CHANNEL_PING
	case params.BRIDGE_ACTION_FILE:
		pc = params.P_CHANNEL_FILE
	default:
		return nil, errors.New("action_not_support")
	}
	rs, e := self.node.Host().NewStream(ctx, tid, pc)
	return rs, e
}

func (self *StreamHandler) pingHandler(s inet.Stream) {
	buf := make([]byte, 4)
	if i, e := s.Read(buf); e != nil {
		log4go.Error(e)
		s.Write([]byte(PANG))
		return
	} else if i != 4 || PING != string(buf) {
		log4go.Error("error : %d , %s", i, string(buf))
		s.Write([]byte(PANG))
		return
	}
	log4go.Info("%s from %s", string(buf), s.Conn().RemotePeer().Pretty())
	t, err := s.Write([]byte(PONG))
	if err != nil {
		log4go.Error("error : %v", err)
	}
	log4go.Info("pong %d", t)
	s.Close()
}

//			 |<-------------------------- head(41) ---------------------------->|
//response = |<- status(1) ->|<- size(8)->|<- file_md5(16) ->|<- part_md5(16) ->|<- file ->|
func (self *StreamHandler) fileHandler(s inet.Stream) {
	var (
		requestSize, rf, rt int64
		buff                []byte
		rid                 = s.Conn().RemotePeer().Pretty()
	)
	defer s.Close()
	//				    |<-------------------- head(24) ------------------->|
	// request_packet = |<- size(8) ->|<- range_from(8) ->|<- range_to(8) ->|<- filepath ->|
	requestHead := make([]byte, 24)
	if i, e := s.Read(requestHead); e != nil {
		log4go.Error(e)
		responseError(s, e.Error())
		return
	} else if i != 24 {
		log4go.Error("error head", i)
		responseError(s, "error head")
		return
	}
	log4go.Info("requestHead -> %v", requestHead)

	sizeBuff := bytes.NewBuffer(requestHead[0:8])
	if err := binary.Read(sizeBuff, binary.BigEndian, &requestSize); err != nil {
		log4go.Error(err)
		responseError(s, err.Error())
		return
	}

	rfBuf := bytes.NewBuffer(requestHead[8:16])
	if err := binary.Read(rfBuf, binary.BigEndian, &rf); err != nil {
		log4go.Error(err)
		responseError(s, err.Error())
		return
	}

	rtBuf := bytes.NewBuffer(requestHead[16:24])
	if err := binary.Read(rtBuf, binary.BigEndian, &rt); err != nil {
		log4go.Error(err)
		responseError(s, err.Error())
		return
	}

	log4go.Info("requestBodySize -> %d , range -> [ %d - %d ]", requestSize, rf, rt)

	if rt < rf {
		log4go.Error("error range [ %d - %d ]", rf, rt)
		responseError(s, "error range")
		return
	}

	buff = make([]byte, requestSize)
	i, e := s.Read(buff)
	if int64(i) != requestSize {
		log4go.Error("result size less than request body size : %d", i)
		responseError(s, "result size less than request body size")
		return
	}
	if e != nil {
		log4go.Error(e)
		responseError(s, e.Error())
		return
	}
	filepath := string(buff)
	filepath = path.Join(params.DataDir, filepath)

	log4go.Info("%s <-- %s", rid, filepath)
	// 如果 rangeFrom 和 rangeTo 都是 0 则表示非 range 传输
	f, err := os.Open(filepath)
	if err != nil {
		log4go.Error(e)
		responseError(s, err.Error())
		return
	}
	defer f.Close()

	// 如果是 range 传输，还要返回当前分片的 hash
	md5sumer := md5.New()
	io.Copy(md5sumer, f)
	filehash := md5sumer.Sum(nil)

	fsize, err := f.Seek(0, 2)
	//filesize, err := f.Seek(0, 2)
	if err != nil {
		log4go.Error(e)
		responseError(s, e.Error())
		return
	}
	filesize := fsize

	// part_md5
	parthash := make([]byte, 16)
	offset := int64(0)
	if rf >= 0 && rt > 0 {
		if rt > filesize {
			err = errors.New("the range_to params can not big then filesize .")
			log4go.Error(err)
			responseError(s, err.Error())
			return
		}
		offset = rf
		filesize = rt - rf

		_, err = f.Seek(offset, 0)
		if err != nil {
			log4go.Error(e)
			responseError(s, e.Error())
			return
		}

		md5sumer = md5.New()
		buff = make([]byte, 4096)
		wt := 0
		for {
			i, err := f.Read(buff)
			if err != nil || i <= 0 {
				log4go.Error(i, err)
				break
			}
			// out of range , split
			if int64(i) > filesize || int64(wt+i) > filesize {
				i = int(filesize - int64(wt))
			}
			t, err := md5sumer.Write(buff[0:i])
			if err != nil {
				log4go.Error(err)
				break
			}
			wt += t
		}
		parthash = md5sumer.Sum(nil)
	}

	_, err = f.Seek(offset, 0)
	if err != nil {
		log4go.Error(e)
		responseError(s, e.Error())
		return
	}

	log4go.Info("<-- hash=%s : size=%d", filehash, filesize)
	//			|<-------------------------- head(41) ---------------------------->|
	//payload = |<- status(1) ->|<- size(8)->|<- file_md5(16) ->|<- part_md5(16) ->|<- file/part ->|
	lenBuff := bytes.NewBuffer([]byte{1})
	binary.Write(lenBuff, binary.BigEndian, fsize)
	//binary.Write(lenBuff, binary.BigEndian, filesize)
	responseHead := lenBuff.Bytes()
	responseHead = append(responseHead, filehash...)
	responseHead = append(responseHead, parthash...)
	log4go.Info("response_head (%d) : %v", len(responseHead), responseHead)
	_, err = s.Write(responseHead)
	if err != nil {
		log4go.Error(err)
		return
	}

	buff = make([]byte, 4096)
	wt := 0
	for {
		i, err := f.Read(buff)
		if err != nil || i <= 0 {
			log4go.Error(i, err)
			break
		}
		// out of range , split
		if int64(i) > filesize || int64(wt+i) > filesize {
			log4go.Warn("⚠️ out of range , split it . before : %d , after : %d", i, filesize-int64(wt))
			i = int(filesize - int64(wt))
		}
		t, err := s.Write(buff[0:i])
		if err != nil {
			log4go.Error(err)
			break
		}
		wt += t
	}

}

func responseError(s inet.Stream, e string) {
	//			|<-------------------------- head(41) ---------------------------->|
	//payload = |<- status(1) ->|<- size(8)->|<- file_md5(16) ->|<- part_md5(16) ->|<- file ->|
	body := []byte(e)
	lenBuff := bytes.NewBuffer([]byte{0})
	ss := int64(len(body))
	binary.Write(lenBuff, binary.BigEndian, ss)
	head := lenBuff.Bytes()
	head = append(head, make([]byte, 32)...)
	payload := append(head, body...)
	log4go.Info(payload)
	s.Write(payload)
}

func isBridgeAction(action byte) bool {
	switch action {
	case params.BRIDGE_ACTION_FILE, params.BRIDGE_ACTION_PING:
		return true
	default:
		return false
	}
}
