package agents

import (
	"bytes"
	"context"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/params"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
)

var (
	ipfsValidatorPacket = func(port int) string {
		return fmt.Sprintf("GET /api/v0/id HTTP/1.1\r\nHost:localhost:%d\r\nUser-Agent:curl/7.54.0\r\nAccept: */*\r\n\r\n", port)
	}
)

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

//TODO rest validator
func (self *Astable) restValidator(id peer.ID) bool {
	return true
}

//TODO web3 validator
func (self *Astable) web3Validator(id peer.ID) bool {
	return false
}

func (self *Astable) ipfsValidator(id peer.ID) bool {
	ctx := context.Background()
	_, port, _, err := IpfsApiAgentConfig.Open()
	if err != nil {
		log4go.Error(err)
		return false
	}
	if self.node.Host().ID() == id {
		return false
	}
	tc, err := self.node.Host().NewStream(ctx, id, params.P_AGENT_IPFS_API)
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
	return bytes.Contains(buf[:t], []byte("HTTP/1.1 200 OK"))
}

func AgentProtoValidator(protoID protocol.ID) bool {
	switch protoID {
	case params.P_AGENT_REST, params.P_AGENT_WEB3_WS, params.P_AGENT_WEB3_RPC, params.P_AGENT_IPFS_API, params.P_AGENT_IPFS_GATEWAY:
		return true
	default:
		return false
	}
}
