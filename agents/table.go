package agents

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/agents/pb"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	"gx/ipfs/QmNtxoGkLeqfM9bsUUe5AdybTPrAUVQmvVctzi92izto9f/go-cookiekit/collections/set"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	"gx/ipfs/QmeuYGSx2wqnfKHWDYAyponLuA9KJSCt8PeUr3ZTpqxAJt/golang-lru"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var (
	flush_intrval_limit  = 120  // sec
	count_as_tab_expired = 300  // sec
	broadcast_expired    = 600  // sec
	as_validator_expired = 1200 // sec
	filter               *set.Set
)

type filterBody struct {
	k string
	f *set.Set
}

func (self *filterBody) Body(pid interface{}, peer peer.ID) *filterBody {
	k := fmt.Sprintf("%s/%s", pid, peer.Pretty())
	return &filterBody{k, filter}
}
func (self *filterBody) Exists() bool {
	return self.f.Exists(self.k)
}
func (self *filterBody) Insert() {
	self.f.Insert(self.k)
}
func (self *filterBody) Remove() {
	self.f.Remove(self.k)
}

// agent-server table
type Astable struct {
	wg               *sync.WaitGroup
	lk               sync.RWMutex
	node             types.Node
	table            map[protocol.ID]*list.List
	intrval          int
	ignoreBroadcast  *set.Set
	countAsTabCache  *lru.Cache
	broadcastCache   *lru.Cache
	asValidatorCache *lru.Cache
}

// TODO loop check Astable and reset latency
/*
type asnode struct {
	Peer peer.ID
	Latency int
}*/

type asValidatorRecord struct {
	id      peer.ID
	protoID protocol.ID
	expired int64 // 1200 sec
	alive   bool
}

func newAsValidatorRecord(protoID protocol.ID, id peer.ID) *asValidatorRecord {
	return &asValidatorRecord{
		id:      id,
		protoID: protoID,
		expired: time.Now().Add(time.Second * time.Duration(as_validator_expired)).Unix(),
	}
}

func (self *asValidatorRecord) String() string {
	return fmt.Sprintf("%v__%v", self.protoID, self.id.Pretty())
}

func (self *asValidatorRecord) Expired() bool {
	if time.Now().Unix() >= self.expired {
		return true
	}
	return false
}
func (self *asValidatorRecord) Alive() bool {
	return self.alive
}
func (self *asValidatorRecord) SetAlive(is bool) {
	self.alive = is
}

type countAsTabRecord struct {
	id      peer.ID
	count   int32
	expired int64 // 300 sec
}

func newCountAsTabRecord(id peer.ID, count int32) *countAsTabRecord {
	return &countAsTabRecord{
		id:      id,
		count:   count,
		expired: time.Now().Add(time.Second * time.Duration(count_as_tab_expired)).Unix(),
	}
}

func (self *countAsTabRecord) Expired() bool {
	if time.Now().Unix() >= self.expired {
		return true
	}
	return false
}

type broadcastRecord struct {
	id      peer.ID
	msgType agents_pb.AgentMessage_Type
	expired int64 // 600 sec
}

func newBroadcastRecord(id peer.ID, t agents_pb.AgentMessage_Type) *broadcastRecord {
	return &broadcastRecord{
		id:      id,
		msgType: t,
		expired: time.Now().Add(time.Second * time.Duration(broadcast_expired)).Unix(),
	}
}

func (self *broadcastRecord) Expired() bool {
	if time.Now().Unix() >= self.expired {
		return true
	}
	return false
}

func (self *broadcastRecord) String() string {
	return fmt.Sprintf("%v__%v", self.id.Pretty(), self.msgType)
}

func NewAstable(node types.Node) *Astable {
	filter = set.New()
	countAsTabCache, _ := lru.New(2000)
	broadcastCache, _ := lru.New(2000)
	asValidatorCache, _ := lru.New(1000)
	tab := make(map[protocol.ID]*list.List)
	wg := new(sync.WaitGroup)
	wg.Add(1) //Wait start done
	return &Astable{
		node:             node,
		table:            tab,
		intrval:          5,
		wg:               wg,
		ignoreBroadcast:  set.New(),
		countAsTabCache:  countAsTabCache,
		broadcastCache:   broadcastCache,
		asValidatorCache: asValidatorCache,
	}
}

func (self *Astable) appendIgnoreBroadcast(p peer.ID) {
	self.lk.Lock()
	defer self.lk.Unlock()
	self.ignoreBroadcast.Insert(p)
}

func (self *Astable) isIgnoreBroadcast(p peer.ID) bool {
	self.lk.RLock()
	defer self.lk.RUnlock()
	return self.ignoreBroadcast.Exists(p)
}

func (self *Astable) GetTable() map[protocol.ID]*list.List {
	return self.table
}

func (self *Astable) Start() {
	defer self.wg.Done()
	log4go.Info("astable start ...")
	go self.loop()

}

func (self *Astable) Append(protoID protocol.ID, peer peer.ID) error {
	if !AgentProtoValidator(protoID) {
		err := fmt.Errorf("👿 agent proto not alow : %s , %v , %s", protoID, []byte(protoID), peer.Pretty())
		log4go.Error(err)
		return err
	}
	self.lk.Lock()
	defer self.lk.Unlock()

	cp := new(filterBody).Body(protoID, peer)
	if cp.Exists() {
		return errors.New(fmt.Sprintf("already exists : %v , %v", protoID, peer))
	}
	defer cp.Insert()
	l, ok := self.table[protoID]
	if !ok {
		l = list.New()
	}
	l.PushFront(peer)
	self.table[protoID] = l
	return nil
}

func (self *Astable) Fetch(protoID protocol.ID) (peer.ID, error) {
	l, ok := self.table[protoID]
	log4go.Info("--> 1 protoID=%v", protoID)
	if ok && l.Len() > 0 {
		log4go.Info("--> 2 protoID=%v, len=%d", protoID, l.Len())
		e := l.Front()
		as := e.Value.(peer.ID)
		log4go.Info("--> 3 protoID=%v, len=%d, as=%v", protoID, l.Len(), as.Pretty())
		if self.AgentServerValidator(protoID, as) {
			defer l.MoveToBack(e)
			return as, nil
		}
		return self.Fetch(protoID)
	}
	return "", errors.New("agent-server not found")
}

func (self *Astable) Remove(protoID protocol.ID, id peer.ID) {
	log4go.Info("🔪 ❌ ---> %s = %s", protoID, id.Pretty())
	self.lk.Lock()
	defer self.lk.Unlock()
	l := self.table[protoID]
	for e := l.Front(); e != nil; e = e.Next() {
		if e.Value.(peer.ID) == id {
			l.Remove(e)
			new(filterBody).Body(protoID, id).Remove()
		}
	}
}

func (self *Astable) loop() {
	timer := time.NewTimer(time.Second * time.Duration(self.intrval))
	resetIntrval := func(t *time.Timer, ast *Astable) {
		if ast.intrval >= flush_intrval_limit {
			t.Reset(time.Second * time.Duration(flush_intrval_limit))
		} else {
			if ast.intrval > flush_intrval_limit/3 {
				ast.intrval += 10
			} else {
				ast.intrval += 5
			}
			t.Reset(time.Second * time.Duration(ast.intrval))
		}
	}
	var (
		prebest  peer.ID
		precount = int32(0)
	)
	// first fetch in peers
	doloop := func() {
		var (
			best          peer.ID
			count         = int32(0)
			wg            = new(sync.WaitGroup)
			sendTask      = set.New()
			totalSendTask = 0
			st            = 50
			conns         = self.node.Host().Network().Conns()
		)
		if len(conns) < st {
			for i := 0; i < len(conns); i++ {
				sendTask.Insert(i)
			}
		} else {
			for i := 0; i < st; i++ {
				sendTask.Insert(rand.Intn(st))
			}
		}

		for i, conn := range conns {
			p := conn.RemotePeer()
			cp := new(filterBody).Body("conns", p)
			if !cp.Exists() {
				cp.Insert()
				protocols, _ := self.node.Host().Peerstore().GetProtocols(p)
				for _, proto := range protocols {
					pid := protocol.ID(proto)
					switch pid {
					case params.P_AGENT_REST, params.P_AGENT_WEB3_RPC, params.P_AGENT_WEB3_WS, params.P_AGENT_IPFS_API, params.P_AGENT_IPFS_GATEWAY:
						log4go.Warn("pid=>%s , peer=>%s", proto, p.Pretty())
						self.Append(pid, p)
					default:
					}
				}
			}

			catc, ok := self.countAsTabCache.Get(p)
			if ok {
				atr := catc.(*countAsTabRecord)
				if atr.Expired() {
					self.countAsTabCache.Remove(p)
				} else {
					continue
				}
			}

			if !self.isIgnoreBroadcast(p) && sendTask.Exists(i) {
				wg.Add(1)
				totalSendTask++
				go func(p peer.ID) {
					defer wg.Done()
					am := agents_pb.NewMessage(agents_pb.AgentMessage_COUNT_AS_TAB)
					respmsg, err := self.SendMsg(context.Background(), p, am)
					if err != nil {
						self.appendIgnoreBroadcast(p)
						return
					}
					defer self.countAsTabCache.Add(p, newCountAsTabRecord(p, respmsg.Count))
					if atomic.LoadInt32(&count) <= respmsg.Count {
						atomic.StoreInt32(&count, respmsg.Count)
						best = p
					}
				}(p)
			}

		}
		wg.Wait()
		log4go.Debug("📣 Agent : COUNT_AS_TAB : total_conns = %d , total_send = %d", len(conns), totalSendTask)

		go func() {
			if count > 0 {
				if best.Pretty() == prebest.Pretty() && count <= precount {
					return
				}
				precount, prebest = count, best
				am := agents_pb.NewMessage(agents_pb.AgentMessage_GET_AS_TAB)
				respmsg, err := self.SendMsg(context.Background(), best, am)
				if err != nil {
					log4go.Error(err)
					return
				}
				for _, as := range respmsg.AgentServerList {
					pid := protocol.ID(as.Pid)
					if !AgentProtoValidator(protocol.ID(pid)) {
						showErrPeers := func(bs [][]byte) {
							for _, b := range bs {
								fmt.Println("->", peer.ID(b).Pretty())
							}
						}
						log4go.Warn("⚠️ skip bad agent proto %v", as.Pid)
						showErrPeers(as.Peers)
						continue
					}
					for _, p := range as.Peers {
						id := peer.ID(p)
						asvr := newAsValidatorRecord(pid, id)
						obj, ok := self.asValidatorCache.Get(asvr.String())
						if ok {
							avr := obj.(*asValidatorRecord)
							if !avr.Alive() && !avr.Expired() {
								log4go.Warn("not allow, %s", avr.String())
								continue
							}
						}
						self.Append(pid, id)
					}
				}
			}
		}()

	}
	for range timer.C {
		resetIntrval(timer, self)
		doloop()
	}
}
