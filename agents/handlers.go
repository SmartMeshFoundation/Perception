package agents

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/agents/pb"
	"github.com/SmartMeshFoundation/Perception/params"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"sync"

	inet "gx/ipfs/QmPtFaR7BWHLAjSwLh9kXcyrgTzDpuhcWLkx8ioa9RMYnx/go-libp2p-net"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	ggio "gx/ipfs/QmdxUuburamoF6zF9qjeQC4WYcWGbWuRmdLacMEsW8ioD8/gogo-protobuf/io"
	//ggio "github.com/gogo/protobuf/io"
	"io"

	"time"
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

type agentsHandler func(context.Context, peer.ID, *agents_pb.AgentMessage) (*agents_pb.AgentMessage, error)

func (self *Astable) HandlerForMsgType(t agents_pb.AgentMessage_Type) agentsHandler {
	switch t {
	case agents_pb.AgentMessage_ADD_AS_TAB:
		return self.addAstab
	case agents_pb.AgentMessage_GET_AS_TAB:
		return self.getAstab
	case agents_pb.AgentMessage_COUNT_AS_TAB:
		return self.countAstab
	default:
		return nil
	}
}

/*

from						bridge				  	to
|							|						|
|---1 AgentMessage_BRIDGE-->|						|
|							|-----2 NewStream------>|
|<--3 AgentMessage_BRIDGE---|						|
|							|						|
|<------------r/w---------->|<----------r/w-------->|
|							|						|

*/

// 针对 agents 搭桥
func (self *Astable) Bridge(ctx context.Context, sc inet.Stream, msg *agents_pb.AgentMessage, handshack func(*agents_pb.AgentMessage) error) error {
	var wg sync.WaitGroup
	log4go.Info("1 AgentMessage_BRIDGE")
	rmsg := agents_pb.NewMessage(agents_pb.AgentMessage_BRIDGE)
	if agents_pb.AgentMessage_BRIDGE != msg.GetType() {
		// 1 error -> 3
		log4go.Info("1 error -> 3 AgentMessage_NULL")
		rmsg = agents_pb.NewMessage(agents_pb.AgentMessage_NULL)
		if err := handshack(rmsg); err != nil {
			return err
		}
		return errors.New("error_type")
	}
	tid := peer.ID(msg.AgentServer.Peers[0])
	pid := protocol.ID(msg.AgentServer.Pid)
	tc, err := self.node.Host().NewStream(ctx, tid, pid)
	log4go.Info("1 -> 2 NewStream to : %s , err = %v", tid.Pretty(), err)
	if err != nil {
		return err
	}
	defer func() {
		if tc != nil {
			tc.Close()
		}
	}()

	err = handshack(rmsg)
	log4go.Info("2 -> 3 AgentMessage_BRIDGE to : %s , err = %v", tid.Pretty(), err)
	if err != nil {
		return err
	}

	// 通道创建成功并且与 from 端完成握手和 to 端链路，开始进行包交换
	wg.Add(2)
	go func() {
		defer wg.Done()
		buf := make([]byte, BufferSize)
		for {
			i, err := sc.Read(buf)
			if err != nil {
				log4go.Error(err)
				tc.Close()
				return
			}

			log4go.Debug("--> in\n\n%s\n\n", buf[:i])

			_, err = tc.Write(buf[:i])
			if err != nil {
				sc.Close()
				log4go.Error(err)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		buf := make([]byte, BufferSize)
		t, u := 0, 0
		for {
			i, err := tc.Read(buf)
			if err != nil {
				log4go.Error(err)
				sc.Close()
				return
			}

			log4go.Debug("<-- out\n\n%s\n\n", buf[:i])

			_, err = sc.Write(buf[:i])
			if err != nil {
				log4go.Error(err)
				tc.Close()
				return
			}
			t += i
			u += 1
			if u%10 == 0 {
				fmt.Println(time.Now().Unix(), "->", t)
			}
		}
	}()
	wg.Wait()

	return nil
}

func (self *Astable) countAstab(ctx context.Context, id peer.ID, msg *agents_pb.AgentMessage) (*agents_pb.AgentMessage, error) {
	msg.Count = 0
	if len(self.table) > 0 {
		for _, l := range self.table {
			msg.Count += int32(l.Len())
		}
	}
	return msg, nil
}

// broadcast this msg to conns and exclude this 'id', ignore if exist .
func (self *Astable) addAstab(ctx context.Context, fromPeer peer.ID, msg *agents_pb.AgentMessage) (*agents_pb.AgentMessage, error) {
	astab, err := agents_pb.AgentMessageToAstab(msg)
	if err != nil {
		return nil, err
	}

	fbCounter := 0
	for p, l := range astab {
		// this moment l.Len == 1
		if l == nil || l.Len() == 0 {
			continue
		}
		e := l.Front()
		pp := e.Value.(peer.ID)
		fb := new(filterBody).Body(string(p), pp)
		if fb.Exists() {
			// ignore
			fbCounter += 1
			continue
		}
		//append to local
		self.Append(p, pp)
	}

	if fbCounter < len(astab) {
		log4go.Info("📢 broadcast msg from -> %s", fromPeer.Pretty())
		self.Broadcast(ctx, fromPeer, msg, true)
	}

	resp := agents_pb.NewMessage(agents_pb.AgentMessage_ADD_AS_TAB)
	return resp, nil
}

// return my astab
func (self *Astable) getAstab(ctx context.Context, id peer.ID, msg *agents_pb.AgentMessage) (*agents_pb.AgentMessage, error) {
	resp, err := agents_pb.AstabToAgentMessage(self.table)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, errors.New("empty msg")
	}
	resp.Type = msg.Type
	return resp, err
}

func (self *Astable) KeepBroadcast(ctx context.Context, from peer.ID, msg *agents_pb.AgentMessage, interval int) {
	for {
		self.Broadcast(ctx, from, msg, false)
		<-time.After(time.Second * time.Duration(interval))
	}
}

//broadcast to conns
func (self *Astable) Broadcast(ctx context.Context, from peer.ID, msg *agents_pb.AgentMessage, duplication bool) {
	self.wg.Wait()
	var (
		conns = self.node.Host().Network().Conns()
		total = 0
		skip  = 0
	)
	for _, conn := range conns {
		rp := conn.RemotePeer()
		br := newBroadcastRecord(rp, msg.GetType())
		history, ok := self.broadcastCache.Get(br.String())
		if !duplication && ok {
			// skip duplication by lru cache
			obj := history.(*broadcastRecord)
			if obj.Expired() {
				self.broadcastCache.Remove(br.String())
			} else {
				skip++
				continue
			}
		}

		if rp != from && !self.isIgnoreBroadcast(rp) {
			total++
			defer self.broadcastCache.Add(br.String(), br)
			go func(p peer.ID) {
				_, err := self.SendMsg(ctx, p, msg)
				if err != nil {
					self.appendIgnoreBroadcast(p)
					return
				}
			}(rp)
		}
	}
	log4go.Info(">> 📢 broadcast %v : total_conns = %d , total_send = %d , skip_duplication = %d", msg.GetType(), len(conns), total, skip)
}

func (self *Astable) SendMsgByStream(ctx context.Context, nstr inet.Stream, msg *agents_pb.AgentMessage) (*agents_pb.AgentMessage, error) {
	var (
		retry = false
		err   error
	)
	for {
		if err != nil {
			return nil, err
		}
		r := ggio.NewDelimitedReader(nstr, inet.MessageSizeMax)
		//w := newBufferedDelimitedWriter(nstr)
		w := ggio.NewDelimitedWriter(nstr)

		err = w.WriteMsg(msg)
		if err != nil {
			nstr.Reset()
			nstr = nil
			if retry {
				log4go.Error("error writing message, bailing: %v", err)
				return nil, err
			} else {
				log4go.Info("error writing message, trying again: %v", err)
				retry = true
				continue
			}
		}

		mes := new(agents_pb.AgentMessage)
		if err := r.ReadMsg(mes); err != nil {
			nstr.Reset()
			nstr = nil
			if retry {
				log4go.Error("error reading message, bailing: %v", err)
				return nil, err
			} else {
				log4go.Info("error reading message, trying again: %v", err)
				retry = true
				continue
			}
		}
		return mes, nil
	}
}

func (self *Astable) SendMsg(ctx context.Context, peer peer.ID, msg *agents_pb.AgentMessage) (*agents_pb.AgentMessage, error) {
	var (
		retry = false
		nstr  inet.Stream
		err   error
	)
	defer func() {
		if nstr != nil {
			nstr.Close()
		}
	}()
	for {
		nstr, err = self.node.Host().NewStream(ctx, peer, params.P_CHANNEL_AGENTS)
		if err != nil {
			return nil, err
		}
		r := ggio.NewDelimitedReader(nstr, inet.MessageSizeMax)
		//w := newBufferedDelimitedWriter(nstr)
		w := ggio.NewDelimitedWriter(nstr)

		err = w.WriteMsg(msg)
		if err != nil {
			nstr.Reset()
			nstr = nil
			if retry {
				log4go.Error("error writing message, bailing: %v", err)
				return nil, err
			} else {
				log4go.Info("error writing message, trying again: %v", err)
				retry = true
				continue
			}
		}

		mes := new(agents_pb.AgentMessage)
		if err := r.ReadMsg(mes); err != nil {
			nstr.Reset()
			nstr = nil
			if retry {
				log4go.Error("error reading message, bailing: %v", err)
				return nil, err
			} else {
				log4go.Info("error reading message, trying again: %v", err)
				retry = true
				continue
			}
		}
		return mes, nil
	}
}
