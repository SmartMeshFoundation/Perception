package agents

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/agents/pb"
	"github.com/SmartMeshFoundation/Perception/params"
	"github.com/SmartMeshFoundation/Perception/tookit"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"net"
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
	case agents_pb.AgentMessage_MY_LOCATION:
		return self.myLocation
	case agents_pb.AgentMessage_YOUR_LOCATION:
		return self.yourLocation
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
	tid := peer.ID(msg.AgentServer.Locations[0].Peer)
	//tid := peer.ID(msg.AgentServer.Peers[0])
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

// ac 在询问邻居时如果发现了 as 时可通过这个协议来询问 as 的 location
func (self *Astable) yourLocation(ctx context.Context, id peer.ID, msg *agents_pb.AgentMessage) (*agents_pb.AgentMessage, error) {
	am := agents_pb.NewMessage(agents_pb.AgentMessage_YOUR_LOCATION)
	if gl := self.node.GetGeoLocation(); gl != nil {
		am.Location = &agents_pb.AgentMessage_Location{Longitude: float32(gl.Longitude), Latitude: float32(gl.Latitude)}
	} else {
		am.Location = &agents_pb.AgentMessage_Location{Longitude: float32(-202), Latitude: float32(-202)}
	}
	return am, nil
}

// ac/as 收到 as 广播时，如果 ac/as 的 location 是空，则通过这个协议来询问 as
func (self *Astable) myLocation(ctx context.Context, id peer.ID, msg *agents_pb.AgentMessage) (*agents_pb.AgentMessage, error) {
	am := agents_pb.NewMessage(agents_pb.AgentMessage_MY_LOCATION)
	am.Location = &agents_pb.AgentMessage_Location{Longitude: float32(-200), Latitude: float32(-200)}
	pi := self.node.Host().Peerstore().PeerInfo(id)
	ips := self.node.GetIP4AddrByMultiaddr(pi.Addrs)

	for _, ip := range ips {
		geodb := self.node.GetGeoipDB()
		if geodb == nil {
			log4go.Error("geodb not started ...")
			return am, nil
		}
		c, err := geodb.City(net.ParseIP(ip))
		// empty city name is private ip addr
		log4go.Info("--> %s : %s, %v", id.Pretty(), ip, c)
		if len(c.Country.Names) == 0 {
			c.Location.Longitude, c.Location.Latitude = -201, -201
		}
		if err == nil && tookit.VerifyLocation(c.Location.Latitude, c.Location.Longitude) {
			am.Location = &agents_pb.AgentMessage_Location{Longitude: float32(c.Location.Longitude), Latitude: float32(c.Location.Latitude)}
			log4go.Info("🌍 🛰️ response MY_LOCATION message : %s (%v)", id.Pretty(), am.Location)
			return am, nil
		}
	}
	return am, nil
}

func (self *Astable) countAstab(ctx context.Context, id peer.ID, msg *agents_pb.AgentMessage) (*agents_pb.AgentMessage, error) {
	msg.Count = 0
	if len(self.table) > 0 {
		for _, l := range self.table {
			msg.Count += int32(len(l))
		}
	}
	return msg, nil
}

// TODO 重灾区，广播风暴
// TODO 重灾区，广播风暴
// TODO 重灾区，广播风暴
// TODO 重灾区，广播风暴
// broadcast this msg to conns and exclude this 'id', ignore if exist .
func (self *Astable) addAstab(ctx context.Context, fromPeer peer.ID, msg *agents_pb.AgentMessage) (*agents_pb.AgentMessage, error) {
	var (
		fbCounter = 0
	)

	astab, err := agents_pb.AgentMessageToAstab(msg)
	if err != nil {
		return nil, err
	}
	//log4go.Info("<<addAstab>> astab_size = %d , from = %s", len(astab), fromPeer)

	// 应该挪到 append 中去;
	// 收到的 as info 也许会带上坐标
	if asl := msg.Location; asl != nil && asl.Peer != nil {
		id := peer.ID(asl.Peer)
		log4go.Info("🌍 🛰️ %s == ( %f, %f ) ", id, asl.Latitude, asl.Longitude)
		tookit.Geodb.Add(id.Pretty(), float64(asl.Latitude), float64(asl.Longitude))
	}

	for p, l := range astab {
		// this moment l.Len == 1
		if l == nil || len(l) == 0 {
			continue
		}
		gl := l[0]
		if gl.ID == "" {
			//log4go.Error("astab_gl_id_nil = %v", gl)
			continue
		}
		// 自己发出去的广播，再传播就是死循环
		if gl.ID == self.node.Host().ID() {
			//log4go.Warn("🤚 refuse receive : died loop : msg.id=%s , myid=%s ", gl.ID.Pretty(), self.node.Host().ID().Pretty())
			break
		}
		fb := new(filterBody).Body(string(p), gl.ID)
		if fb.Exists() {
			// ignore
			fbCounter += 1
			continue
		}
		if l0, ok := tookit.Geodb.GetNode(gl.ID.Pretty()); ok && tookit.VerifyLocation(l0.Latitude, l0.Longitude) {
			// ignore
			fbCounter += 1
			continue
		}

		//append to local
		log4go.Info("addAstab -> astab.Append : %s -> , %s : %s , %d", fromPeer, p, gl.ID, len(astab))
		self.Append(p, gl)

		// TODO 如果 selfgeo 是空，在这里要问去 as 问一次 selfgeo
		if self.node.GetGeoLocation() == nil {
			params.AACh <- params.NewAA(params.AA_GET_MY_LOCATION, nil)
		}

	}

	if fbCounter < len(astab) {

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
		conns  = self.node.Host().Network().Conns()
		total  = 0
		skip   = 0
		source peer.ID
	)

	// 获取消息源头 >>>>>
	astabMsg, err := agents_pb.AgentMessageToAstab(msg)
	if err != nil {
		log4go.Error(err)
		return
	}
	for _, l := range astabMsg {
		if len(l) > 0 {
			gl := l[0]
			source = gl.ID
			break
		}
	}
	if source == "" {
		//log4go.Error("-> skip_source_nil : from=%v ", from)
		return
	}
	// 获取消息源头 <<<<<

	for _, conn := range conns {
		remotePeer := conn.RemotePeer()
		br := newBroadcastRecord(remotePeer, msg.GetType())

		history, ok := self.broadcastCache.Get(br.String())
		if !duplication && ok {
			// skip duplication by lru cache
			obj := history.(*broadcastRecord)
			if obj.Expired() {
				self.broadcastCache.Remove(br.String())
			} else {
				skip++
				//log4go.Info("-> skip_broadcast -> rp=%s, src=%s, from=%s", remotePeer, source, from)
				continue
			}
		}

		// 不能发给消息发送人，也不能发给消息源
		if remotePeer != source && remotePeer != from {
			self.broadcastCache.Add(br.String(), br)
			total++
			self.broadcastCh <- BroadcastMsg{remotePeer, msg}
			log4go.Debug("📢 broadcast msg from -> %s", remotePeer)
		} else {
			log4go.Info("-> 🚥 skip_broadcast -> remotePeer,source,from = %s,%s,%s ", remotePeer, source, from)
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
		if r == nil {
			return nil, errors.New("ggio_reader_nil")
		}
		w := ggio.NewDelimitedWriter(nstr)
		if w == nil {
			return nil, errors.New("ggio_writer_nil")
		}

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
