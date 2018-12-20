package agents_pb

import (
	"container/list"
	"errors"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
)

func NewMessage(t AgentMessage_Type) *AgentMessage {
	msg := new(AgentMessage)
	msg.Type = t
	msg.Count = 0
	return msg
}

func (self *AgentMessage) Append(protoID protocol.ID, peer peer.ID) *AgentMessage {
	as := new(AgentMessage_AgentServer)
	as.Pid = []byte(protoID)
	as.Peers = [][]byte{[]byte(peer)}
	if self.AgentServerList == nil || len(self.AgentServerList) == 0 {
		self.AgentServerList = []*AgentMessage_AgentServer{as}
	} else {
		self.AgentServerList = append(self.AgentServerList[:], as)
	}
	return self
}

func AstabToAgentMessage(table map[protocol.ID]*list.List) (*AgentMessage, error) {
	if table == nil || len(table) < 1 {
		return nil, errors.New("empty astable")
	}
	msg := new(AgentMessage)
	msg.AgentServerList = make([]*AgentMessage_AgentServer, len(table))
	i := 0
	for p, l := range table {
		as := new(AgentMessage_AgentServer)
		as.Pid = []byte(p)
		if l == nil || l.Len() == 0 {
			continue
		}
		as.Peers = make([][]byte, l.Len())
		j := 0
		for e := l.Front(); e != nil; e = e.Next() {
			pp := e.Value.(peer.ID)
			as.Peers[j] = []byte(pp)
			j += 1
		}
		msg.AgentServerList[i] = as
		i += 1
	}
	return msg, nil
}

func AgentMessageToAstab(msg *AgentMessage) (map[protocol.ID]*list.List, error) {
	fn := func() (map[protocol.ID]*list.List, error) {
		astab := make(map[protocol.ID]*list.List)
		for _, as := range msg.AgentServerList {
			pid := protocol.ID(as.Pid)
			l, ok := astab[pid]
			if !ok {
				l = list.New()
			}
			bpeers := as.Peers
			for _, bpeer := range bpeers {
				peer := peer.ID(bpeer)
				l.PushFront(peer)
			}
			astab[pid] = l
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
