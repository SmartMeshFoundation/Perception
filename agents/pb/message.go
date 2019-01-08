package agents_pb

import (
	"errors"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
)

func NewMessage(t AgentMessage_Type) *AgentMessage {
	msg := new(AgentMessage)
	msg.Type = t
	msg.Count = 0
	return msg
}

func (self *AgentMessage) Append(protoID protocol.ID, location *types.GeoLocation) *AgentMessage {
	as := new(AgentMessage_AgentServer)
	as.Pid = []byte(protoID)
	as.Locations = []*AgentMessage_Location{NewAgentLocation(location.ID, location.Latitude, location.Longitude)}
	//as.Peers = [][]byte{[]byte(peer)}

	if self.AgentServerList == nil || len(self.AgentServerList) == 0 {
		self.AgentServerList = []*AgentMessage_AgentServer{as}
	} else {
		self.AgentServerList = append(self.AgentServerList[:], as)
	}
	return self
}

func AstabToAgentMessage(table map[protocol.ID][]*types.GeoLocation) (*AgentMessage, error) {
	if table == nil || len(table) < 1 {
		return nil, errors.New("empty astable")
	}
	msg := new(AgentMessage)
	msg.AgentServerList = make([]*AgentMessage_AgentServer, len(table))
	i := 0
	for p, l := range table {
		as := new(AgentMessage_AgentServer)
		as.Pid = []byte(p)
		if l == nil || len(l) == 0 {
			continue
		}
		as.Locations = make([]*AgentMessage_Location, len(l))
		for j, g := range l {
			as.Locations[j] = NewAgentLocation(g.ID, g.Latitude, g.Longitude)
		}
		msg.AgentServerList[i] = as
		i += 1
	}
	return msg, nil
}

func AgentMessageToAstab(msg *AgentMessage) (map[protocol.ID][]*types.GeoLocation, error) {
	fn := func() (map[protocol.ID][]*types.GeoLocation, error) {
		astab := make(map[protocol.ID][]*types.GeoLocation)
		for _, as := range msg.AgentServerList {
			pid := protocol.ID(as.Pid)
			l, ok := astab[pid]
			if !ok {
				l = make([]*types.GeoLocation, 0)
			}

			if len(as.Locations) > 0 {
				for _, location := range as.Locations {
					peer := peer.ID(location.Peer)
					g := types.NewGeoLocation(float64(location.Longitude), float64(location.Latitude))
					g.ID = peer
					l = append(l, g)
				}
				astab[pid] = l
			}
			/*
				bpeers := as.Peers
				for _, bpeer := range bpeers {
					peer := peer.ID(bpeer)
					l.PushFront(peer)
				}
				astab[pid] = l
			*/
		}
		return astab, nil
	}

	switch msg.Type {
	case AgentMessage_GET_AS_TAB, AgentMessage_ADD_AS_TAB:
		return fn()
	default:
		return nil, errors.New("error type")
	}
}

func NewAgentLocation(id peer.ID, lat, lng float64) *AgentMessage_Location {
	al := &AgentMessage_Location{}
	al.Latitude = float32(lat)
	al.Longitude = float32(lng)
	al.Peer = []byte(id)
	return al
}
