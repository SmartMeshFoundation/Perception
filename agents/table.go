package agents

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/agents/pb"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	"github.com/SmartMeshFoundation/Perception/tookit"
	"gx/ipfs/QmNtxoGkLeqfM9bsUUe5AdybTPrAUVQmvVctzi92izto9f/go-cookiekit/collections/set"
	inet "gx/ipfs/QmPtFaR7BWHLAjSwLh9kXcyrgTzDpuhcWLkx8ioa9RMYnx/go-libp2p-net"
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
	table            map[protocol.ID]*list.List // list<*types.GeoLocation>
	intrval          int
	ignoreBroadcast  *set.Set
	countAsTabCache  *lru.Cache
	broadcastCache   *lru.Cache
	asValidatorCache *lru.Cache
	//geodbAddCh       chan peer.ID
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
func (self *Astable) GenBridge(ctx context.Context, bid, tid peer.ID, pid protocol.ID) (inet.Stream, error) {
	log4go.Info("-[ 👬 gen agent bridge 👬 ]-> (%s) --> (%s) ", bid.Pretty(), tid.Pretty())
	am := agents_pb.NewMessage(agents_pb.AgentMessage_BRIDGE)
	as := new(agents_pb.AgentMessage_AgentServer)
	as.Pid = []byte(pid)

	as.Locations = []*agents_pb.AgentMessage_Location{agents_pb.NewAgentLocation(tid, 0, 0)}
	//as.Peers = [][]byte{[]byte(tid)}

	am.AgentServer = as
	stream, err := self.node.Host().NewStream(ctx, bid, params.P_CHANNEL_AGENTS)
	if err != nil {
		return nil, err
	}
	// 1
	rm, err := self.SendMsgByStream(ctx, stream, am)
	if err != nil {
		return nil, err
	}
	// 3
	if rm.Type == agents_pb.AgentMessage_BRIDGE {
		log4go.Info("agent_bridge_handshack_success %s : %s -> %s", pid, bid.Pretty(), tid.Pretty())
		return stream, nil
	}
	return nil, errors.New("error type")
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
		//geodbAddCh:       make(chan peer.ID, 512),
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
	//go self.geodbAdd()
}

/*
func (self *Astable) geodbAdd() {
	ctx := context.Background()
	for p := range self.geodbAddCh {
		ascr := newAsValidatorRecord(params.P_AGENT_IPFS_API, p)
		if obj, ok := self.asValidatorCache.Get(ascr.String()); ok {
			if asr := obj.(*asValidatorRecord); !asr.Expired() && !asr.Alive() {
				//fmt.Println("---- ignor ----> ", p.Pretty())
				continue
			}
		}
		if _, ok := tookit.Geodb.GetNode(p.Pretty()); !ok {
			pi, err := self.node.FindPeer(ctx, p, nil)
			if err == nil {
				// TODO
				ips := tookit.GetIP4AddrByMultiaddr(pi.Addrs)
				for _, ip := range ips {
					c, err := self.node.GetGeoipDB().City(net.ParseIP(ip))
					if err == nil && c.Location.Longitude > 0 && c.Location.Latitude > 0 {
						tookit.Geodb.Add(p.Pretty(), c.Location.Latitude, c.Location.Longitude)
					}
				}
			} else {
				// 删除无法找到的节点，并且拒绝再次同步这个节点
				//same validator
				log4go.Info(" remove peer when can not find : %s", p.Pretty())
				self.RemoveAll(p)
			}
		}
	}
}
*/

func (self *Astable) Append(protoID protocol.ID, location *types.GeoLocation) error {

	if !AgentLocationValidator(location) {
		err := fmt.Errorf("👿 agent location empty : %s , %v , %s", protoID, []byte(protoID), location.ID.Pretty())
		log4go.Error(err)
		return err
	}

	if !AgentProtoValidator(protoID) {
		err := fmt.Errorf("👿 agent proto not alow : %s , %v , %s", protoID, []byte(protoID), location.ID.Pretty())
		log4go.Error(err)
		return err
	}

	self.lk.Lock()
	defer func() {
		self.lk.Unlock()
	}()

	cp := new(filterBody).Body(protoID, location.ID)
	if cp.Exists() {
		return errors.New(fmt.Sprintf("already exists : %v , %v", protoID, location.ID))
	}

	defer cp.Insert()
	l, ok := self.table[protoID]
	if !ok {
		l = list.New()
	}
	l.PushFront(location)
	self.table[protoID] = l
	return nil
}

func (self *Astable) fetchWithOutGeo(protoID protocol.ID) (peer.ID, error) {
	log4go.Info("<<FetchAs>> fetch-without-geo 🌍")
	l, ok := self.table[protoID]
RETRY:
	if ok && l.Len() > 0 {
		e := l.Front()
		as, ok := e.Value.(*types.GeoLocation)
		if !ok {
			l.Remove(e)
			goto RETRY
		}
		//as := e.Value.(peer.ID)
		if self.AgentServerValidator(protoID, as.ID) {
			defer l.MoveToBack(e)
			return as.ID, nil
		}
		return self.fetchWithOutGeo(protoID)
	}
	return "", errors.New("agent-server not found")
}

func (self *Astable) fetchWithGeo(protoID protocol.ID) (peer.ID, error) {
	log4go.Info("<<FetchAs>> fetch-with-geo 🚩️")
	selfgeo := self.node.GetGeoLocation()
	if selfgeo == nil {
		return "", errors.New("self geo not found.")
	}
	l, ok := self.table[protoID]
	if ok && l.Len() > 0 {
		gll := make([]*types.GeoLocation, 0)
		for e := l.Front(); e != nil; e = e.Next() {
			as, ok := e.Value.(*types.GeoLocation)
			if !ok {
				l.Remove(e)
				continue
			}
			gll = append(gll, as)
		}
		rll, ok := tookit.Geodb.FilterNode(selfgeo, gll)
		if !ok {
			return self.fetchWithOutGeo(protoID)
		}
		i := 0
		if size := len(rll); size > 1 {
			i = rand.Intn(size)
		}
		p := rll[i].ID
		log4go.Info("<<FetchAs>> fetch-with-geo-success : total=%d, i=%d, target=%s", len(rll), i, p.Pretty())
		if self.AgentServerValidator(protoID, p) {
			return p, nil
		}
		return self.fetchWithGeo(protoID)
	}
	return "", errors.New("agent-server not found")
}

func (self *Astable) Fetch(protoID protocol.ID) (peer.ID, error) {
	if self.node.GetGeoLocation() == nil {
		return self.fetchWithOutGeo(protoID)
	}
	return self.fetchWithGeo(protoID)
}

// 这个方法只有检查 ip 坐标的地方在调用，所以被触发时一定是目标节点无效
func (self *Astable) RemoveAll(id peer.ID) {
	for _, p := range params.P_AGENT_ALL {
		ascr := newAsValidatorRecord(p, id)
		ascr.SetAlive(false)
		self.asValidatorCache.Add(ascr.String(), ascr)
		self.Remove(p, id)
	}
}
func (self *Astable) Remove(protoID protocol.ID, id peer.ID) {
	self.lk.Lock()
	defer func() {
		tookit.Geodb.Delete(id.Pretty())
		self.lk.Unlock()
	}()
	l := self.table[protoID]
	if l == nil || l.Len() == 0 {
		return
	}
	for e := l.Front(); e != nil; e = e.Next() {
		if gl, ok := e.Value.(*types.GeoLocation); ok && gl.ID == id {
			l.Remove(e)
			log4go.Info("🔪 ❌ ---> %s = %s", protoID, id.Pretty())
			new(filterBody).Body(protoID, id).Remove()
		} else if !ok {
			l.Remove(e)
		}
	}
}

func (self *Astable) QuerySelfLocation(target peer.ID) {
	if self.node.GetGeoLocation() == nil {
		req := agents_pb.NewMessage(agents_pb.AgentMessage_MY_LOCATION)
		resp, err := self.SendMsg(context.Background(), target, req)
		log4go.Info("<<selfgeoFn>> my_location_response : %v , %v", err, resp)
		if err != nil {
			log4go.Error("🛰️ 🌍 get_my_location error : %v", err)
			return
		}
		if !tookit.VerifyLocation(resp.Location.Latitude, resp.Location.Longitude) {
			log4go.Error("🛰️ 🌍 get_my_location fail : %v", resp.Location)
			return
		}
		gl := types.NewGeoLocation(float64(resp.Location.Longitude), float64(resp.Location.Latitude))
		gl.ID = self.node.Host().ID()
		self.node.SetGeoLocation(gl)
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
		/*		selfgeoFn = func(as peer.ID) {
				if self.node.GetGeoLocation() == nil {
					req := agents_pb.NewMessage(agents_pb.AgentMessage_MY_LOCATION)
					resp, err := self.SendMsg(context.Background(), as, req)
					log4go.Info("<<selfgeoFn>> my_location_response : %v , %v", err, resp)
					if err != nil {
						log4go.Error("🛰️ 🌍 get_my_location error : %v", err)
						return
					}
					if resp.Location.Latitude == 0 {
						log4go.Error("🛰️ 🌍 get_my_location fail : %v", resp.Location)
						return
					}
					gl := types.NewGeoLocation(float64(resp.Location.Longitude), float64(resp.Location.Latitude))
					self.node.SetGeoLocation(gl)
				}
			}*/
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

		// 在邻居列表里寻找 as
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

						ctx := context.Background()
						req := agents_pb.NewMessage(agents_pb.AgentMessage_YOUR_LOCATION)
						resp, err := self.SendMsg(ctx, p, req)
						if err == nil && tookit.VerifyLocation(resp.Location.Latitude, resp.Location.Longitude) {
							gl := types.NewGeoLocation(float64(resp.Location.Longitude), float64(resp.Location.Latitude))
							gl.ID = p
							self.QuerySelfLocation(p)
							self.Append(pid, gl)
						}
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

		// 周期到邻居节点去获取 astab , 只去 best 节点获取
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
						log4go.Warn("⚠️ skip bad agent proto %v", as.Pid)
						continue
					}
					for _, location := range as.Locations {
						id := peer.ID(location.Peer)
						asvr := newAsValidatorRecord(pid, id)
						obj, ok := self.asValidatorCache.Get(asvr.String())
						if ok {
							avr := obj.(*asValidatorRecord)
							if !avr.Alive() && !avr.Expired() {
								log4go.Warn("not allow, %s", avr.String())
								continue
							}
						}
						tookit.Geodb.Add(id.Pretty(), float64(location.Latitude), float64(location.Longitude))
						gl := types.NewGeoLocation(float64(location.Longitude), float64(location.Latitude))
						gl.ID = id
						self.Append(pid, gl)

					}
					/*
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
					*/
				}
			}
		}()

	}
	for range timer.C {
		resetIntrval(timer, self)
		doloop()
	}
}
