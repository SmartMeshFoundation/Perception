package agents

import (
	"bytes"
	"context"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	inet "gx/ipfs/QmPtFaR7BWHLAjSwLh9kXcyrgTzDpuhcWLkx8ioa9RMYnx/go-libp2p-net"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	"time"
)

var (
	ipfsValidatorPacket = func(port int) string {
		return fmt.Sprintf("GET /api/v0/id HTTP/1.1\r\nHost:localhost:%d\r\nUser-Agent:curl/7.54.0\r\nAccept: */*\r\n\r\n", port)
	}
	restValidatorPacket = func(port int) string {
		return fmt.Sprintf("GET /vcoin/api/ping HTTP/1.1\r\nHost:localhost:%d\r\nUser-Agent:curl/7.54.0\r\nAccept: */*\r\n\r\n", port)
	}
)

type avt struct {
	idx     int
	ctx     context.Context
	protoID protocol.ID
	id      peer.ID
	rtn     chan peer.ID
}

func (self *Astable) AsyncAgentServerValidator(ctx context.Context, protoID protocol.ID, gll []*types.GeoLocation, rtn chan peer.ID) {
	for i, gl := range gll {
		go func(a avt) {
			log4go.Debug("✨ ---> %d", a.idx)
			select {
			case <-a.ctx.Done():
				log4go.Debug("😢 ---> %d", a.idx)
				return
			default:
				defer log4go.Debug("🌛 <--- %d", a.idx)
				if self.AgentServerValidator(a.protoID, a.id) {
					a.rtn <- a.id
				} else {
					a.rtn <- peer.ID("")
				}
				return
			}
		}(avt{i, ctx, protoID, peer.ID(gl.ID), rtn})
	}
}

//TODO check as
func (self *Astable) AgentServerValidator(protoID protocol.ID, as peer.ID) bool {
	ascr := newAsValidatorRecord(protoID, as)
	if obj, ok := self.asValidatorCache.Get(ascr.String()); ok {
		if asr := obj.(*asValidatorRecord); !asr.Expired() && asr.Alive() {
			return true
		} else if asr.Expired() {
			self.asValidatorCache.Remove(ascr.String())
		} else if !asr.Alive() {
			return false
		}
	}
	switch protoID {
	case params.P_AGENT_REST:
		ok := self.restValidator(as)
		ascr.SetAlive(ok)
		defer self.asValidatorCache.Add(ascr.String(), ascr)
		if !ok {
			self.Remove(params.P_AGENT_REST, as)
		}
		return ok
	case params.P_AGENT_IPFS_API, params.P_AGENT_IPFS_GATEWAY:
		ok := self.ipfsValidator(as)
		log4go.Debug(" 👮‍ IPFS_VALIDATOR : %v : %s", ok, as.Pretty())
		ascr.SetAlive(ok)
		defer self.asValidatorCache.Add(ascr.String(), ascr)
		if !ok {
			self.Remove(params.P_AGENT_IPFS_API, as)
			self.Remove(params.P_AGENT_IPFS_GATEWAY, as)
		}
		return ok
	case params.P_AGENT_WEB3_RPC, params.P_AGENT_WEB3_WS:
		ok := self.web3Validator(as)
		ascr.SetAlive(ok)
		defer self.asValidatorCache.Add(ascr.String(), ascr)
		if !ok {
			self.Remove(params.P_AGENT_WEB3_RPC, as)
			self.Remove(params.P_AGENT_WEB3_WS, as)
		}
		return ok
	default:
		self.Remove(protoID, as)
		return false
	}
}

func (self *Astable) restValidator(id peer.ID) bool {
	var (
		tc      inet.Stream
		ctx     = context.Background()
		findby  = make(chan peer.ID)
		s       = time.Now().Unix()
		timeout = 5 * time.Second
	)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer func() {
		if tc != nil {
			tc.Close()
		}
		cancel()
	}()
	_, port, _, err := IpfsApiAgentConfig.Open()
	if err != nil {
		log4go.Error(err)
		return false
	}
	if self.node.Host().ID() == id {
		return false
	}
	pi, err := self.node.FindPeer(ctx, id, findby)

	if err != nil {
		log4go.Debug("<<restValidator-FindPeer>> error=%s, time_used=%ds", err, time.Now().Unix()-s)
		return false
	}
	log4go.Debug("<<restValidator-FindPeer>> success , time_used=%ds", time.Now().Unix()-s)
	// 尝试直连
	if err = self.node.Host().Connect(ctx, pi); err != nil {
		// 尝试搭桥 这里一定有返回值
		fy := <-findby
		if fy == "" {
			return false
		}
		tc, err = self.GenBridge(ctx, fy, id, params.P_AGENT_REST)
		log4go.Debug(" <<restValidator-Bridge>> %v : %s --> %s , time_used=%ds", err, fy, id, time.Now().Unix()-s)
	} else {
		tc, err = self.node.Host().NewStream(ctx, id, params.P_AGENT_REST)
		log4go.Debug(" <<restValidator-Normal>> %v : --> %s , time_used=%ds", err, id, time.Now().Unix()-s)
	}

	if err != nil {
		log4go.Error(err)
		return false
	}

	packet := restValidatorPacket(port)
	_, err = tc.Write([]byte(packet))
	if err != nil {
		log4go.Error(err)
		return false
	}
	buf := make([]byte, 2048)
	t, err := tc.Read(buf)
	//vr := bytes.Contains(buf[:t], []byte("HTTP/1.1 200 OK"))
	//sr := strings.Contains(string(buf[:t]), "HTTP/1.1 200 OK")
	//fmt.Printf("rest-validator ==> \n %s \n err=%v, vr = %v , sr = %v \n", buf[:t], err, vr, sr)
	if err != nil {
		log4go.Error(err)
		return false
	}
	res := bytes.Contains(buf[:t], []byte("HTTP/1.1 200"))
	log4go.Info(" <<restValidator>> done : %v , %s, time_used=%ds", res, id, time.Now().Unix()-s)
	return res
}

//TODO web3 validator
func (self *Astable) web3Validator(id peer.ID) bool {
	return false
}

func (self *Astable) ipfsValidator(id peer.ID) bool {
	var (
		tc      inet.Stream
		ctx     = context.Background()
		findby  = make(chan peer.ID)
		s       = time.Now().Unix()
		timeout = 3 * time.Second
		p       = params.P_AGENT_IPFS_API
	)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer func() {
		if tc != nil {
			tc.Close()
		}
		cancel()
	}()

	_, port, _, err := IpfsApiAgentConfig.Open()
	if err != nil {
		log4go.Error(err)
		return false
	}
	if self.node.Host().ID() == id {
		return false
	}

	pi, err := self.node.FindPeer(ctx, id, findby)

	if err != nil {
		log4go.Debug("<<ipfsValidator-FindPeer>> error=%s, time_used=%ds", err, time.Now().Unix()-s)
		return false
	}
	// 尝试直连
	if err = self.node.Host().Connect(ctx, pi); err != nil {
		// 尝试搭桥 这里一定有返回值
		fy := <-findby
		if fy == "" {
			return false
		}
		tc, err = self.GenBridge(ctx, fy, id, p)
		log4go.Debug("<<ipfsValidator-FindPeer>> 🌈 success , time_used=%ds", time.Now().Unix()-s)
	} else {
		tc, err = self.node.Host().NewStream(ctx, id, p)
		log4go.Debug("<<ipfsValidator-FindPeer>> 🌞 success , time_used=%ds", time.Now().Unix()-s)
	}
	//tc, err := self.node.Host().NewStream(ctx, id, p)

	if err != nil {
		log4go.Error(err)
		return false
	}
	defer func() {
		if tc != nil {
			tc.Close()
		}
	}()
	packet := ipfsValidatorPacket(port)
	_, err = tc.Write([]byte(packet))
	if err != nil {
		log4go.Error(err)
		return false
	}
	buf := make([]byte, 2048)
	t, err := tc.Read(buf)
	if err != nil {
		log4go.Error(err)
		return false
	}
	return bytes.Contains(buf[:t], []byte("HTTP/1.1 200"))
}

func AgentLocationValidator(location *types.GeoLocation) bool {
	return true
	/*	if location.Latitude > 0 && location.Longitude > 0 {
			return true
		}
		return false*/
}

func AgentProtoValidator(protoID protocol.ID) bool {
	switch protoID {
	case params.P_AGENT_REST, params.P_AGENT_WEB3_WS, params.P_AGENT_WEB3_RPC, params.P_AGENT_IPFS_API, params.P_AGENT_IPFS_GATEWAY:
		return true
	default:
		return false
	}
}
