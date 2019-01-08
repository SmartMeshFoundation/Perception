package agents

import (
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
	k  string
	f  *set.Set
	lk sync.RWMutex
}

func (self *filterBody) Body(pid interface{}, peer peer.ID) *filterBody {
	k := fmt.Sprintf("%s/%s", pid, peer.Pretty())
	return &filterBody{k: k, f: filter}
}
func (self *filterBody) Exists() bool {
	self.lk.RLock()
	defer self.lk.RUnlock()
	return self.f.Exists(self.k)
}
func (self *filterBody) Insert() {
	self.lk.Lock()
	defer self.lk.Unlock()
	self.f.Insert(self.k)
}
func (self *filterBody) Remove() {
	self.lk.Lock()
	defer self.lk.Unlock()
	self.f.Remove(self.k)
}

type BroadcastMsg struct {
	to  peer.ID
	msg *agents_pb.AgentMessage
}

// agent-server table
type Astable struct {
	wg    *sync.WaitGroup
	lk    *sync.RWMutex
	node  types.Node
	table map[protocol.ID][]*types.GeoLocation // list<*types.GeoLocation>
	//table            map[protocol.ID]list.List // list<*types.GeoLocation>
	intrval          int
	ignoreBroadcast  *set.Set
	countAsTabCache  *lru.Cache
	broadcastCache   *lru.Cache
	asValidatorCache *lru.Cache
	broadcastCh      chan BroadcastMsg
	//geodbAddCh       chan peer.ID
	// 记录上一次发送的 AgentMessage_COUNT_AS_TAB 消息 >>>
	prebest, best peer.ID
	precount      int32
	// 记录上一次发送的 AgentMessage_COUNT_AS_TAB 消息 <<<
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

	as.Locations = []*agents_pb.AgentMessage_Location{agents_pb.NewAgentLocation(tid, -205, -205)}
	//as.Peers = [][]byte{[]byte(tid)}

	am.AgentServer = as
	stream, err := self.node.Host().NewStream(ctx, bid, params.P_CHANNEL_AGENTS)
	if err != nil {
		return nil, err
	}
	// 1
	rm, err := self.SendMsgByStream(ctx, stream, am)
	if err != nil {
		log4go.Error("GenBridge_SendMsgByStream_err: %s", err)
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
	tab := make(map[protocol.ID][]*types.GeoLocation)
	//tab := make(map[protocol.ID]*list.List)
	wg := new(sync.WaitGroup)
	wg.Add(1) //Wait start done
	return &Astable{
		lk:               new(sync.RWMutex),
		node:             node,
		table:            tab,
		intrval:          5,
		wg:               wg,
		ignoreBroadcast:  set.New(),
		countAsTabCache:  countAsTabCache,
		broadcastCache:   broadcastCache,
		asValidatorCache: asValidatorCache,
		broadcastCh:      make(chan BroadcastMsg, 16),
		//geodbAddCh:       make(chan peer.ID, 512),
		best:     peer.ID(""),
		prebest:  peer.ID(""),
		precount: 0,
	}
}

func (self *Astable) appendIgnoreBroadcast(p interface{}) {
	self.lk.Lock()
	defer self.lk.Unlock()
	self.ignoreBroadcast.Insert(p)
}

func (self *Astable) isIgnoreBroadcast(p interface{}) bool {
	self.lk.RLock()
	defer self.lk.RUnlock()
	return self.ignoreBroadcast.Exists(p)
}

//func (self *Astable) GetTable() map[protocol.ID]*list.List {
func (self *Astable) GetTable() map[protocol.ID][]*types.GeoLocation {
	return self.table
}

func (self *Astable) Start() {
	defer self.wg.Done()
	log4go.Info("astable start ...")
	go self.loop()
	go self.loopBroadcast()
	//go self.geodbAdd()
}

func (self *Astable) loopBroadcast() {
	for bm := range self.broadcastCh {
		_, err := self.SendMsg(context.Background(), bm.to, bm.msg)
		log4go.Info("broadcast -> err=%v , %s", err, bm.to.Pretty())
	}
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
		err := fmt.Errorf("agent location empty : %s , %v , %s", protoID, []byte(protoID), location.ID.Pretty())
		//log4go.Error(err)
		return err
	}

	if len(location.ID) == 0 {
		err := fmt.Errorf("agent id empty : %s , %v , %v", protoID, []byte(protoID), location)
		//log4go.Error(err)
		return err
	}

	if !AgentProtoValidator(protoID) {
		err := fmt.Errorf("agent proto not alow : %s , %v , %s", protoID, []byte(protoID), location.ID.Pretty())
		//log4go.Error(err)
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
	//defer cp.Insert()

	if tookit.VerifyLocation(location.Latitude, location.Longitude) {
		defer cp.Insert()
	} else {
		//加入待处理任务，等待重置
		params.AACh <- params.NewAA(params.AA_GET_AS_LOCATION, location.ID)
	}

	l, ok := self.table[protoID]
	if !ok {
		l = make([]*types.GeoLocation, 0)
	} else {
		// 去掉重复的
		for i, g := range l {
			if g.ID == location.ID {
				l = append(l[:i], l[i+1:]...)
			}
		}
	}

	/*	else {
		// 去掉重复的
		for e := l.Front(); e != nil; e = e.Next() {
			as, ok := e.Value.(*types.GeoLocation)
			if !ok || as.ID == location.ID {
				l.Remove(e)
			}
		}
	}*/
	l = append(l, location)
	//l.PushFront(location)
	self.table[protoID] = l
	return nil
}

func (self *Astable) fetchWithOutGeo(protoID protocol.ID) (peer.ID, error) {
	log4go.Info("<<FetchAs>> fetch-without-geo 🌍")
	l, ok := self.table[protoID]
	if ok && len(l) > 0 {
		// 随机
		as := l[rand.Intn(len(l))]
		if self.AgentServerValidator(protoID, as.ID) {
			return as.ID, nil
		}
		return self.fetchWithOutGeo(protoID)
	}
	/*	if ok && l.Len() > 0 {
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
	}*/
	return "", errors.New("agent-server not found")
}

func (self *Astable) fetchWithGeo(protoID protocol.ID) (peer.ID, error) {
	log4go.Info("<<FetchAs>> fetch-with-geo 🚩 🚩️")
	selfgeo := self.node.GetGeoLocation()
	if selfgeo == nil {
		return "", errors.New("self geo not found.")
	}
	l, ok := self.table[protoID]
	if ok && len(l) > 0 {
		rll, ok := tookit.Geodb.FilterNode(selfgeo, l)
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
	/*	if ok && l.Len() > 0 {
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
	}*/
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
	if l == nil || len(l) == 0 {
		return
	}
	for i, gl := range l {
		if gl.ID == id || gl.ID == "" {
			self.table[protoID] = append(l[:i], l[i+1:]...)
			log4go.Info("🔪 astab_remove ---> %s = %s", protoID, id.Pretty())
			new(filterBody).Body(protoID, id).Remove()
		}
	}
	/*	for e := l.Front(); e != nil; e = e.Next() {
		if gl, ok := e.Value.(*types.GeoLocation); ok && gl.ID == id {
			l.Remove(e)
			new(filterBody).Body(protoID, id).Remove()
		} else if !ok {
			l.Remove(e)
		}
	}*/
}

func (self *Astable) Reset(location *types.GeoLocation) {
	id := location.ID
	log4go.Info("🏯 --->", id.Pretty(), location)
	self.lk.Lock()
	defer func() {
		tookit.Geodb.Add(id.Pretty(), location.Latitude, location.Longitude)
		self.lk.Unlock()
	}()

	for _, protoID := range params.P_AGENT_ALL {
		l := self.table[protoID]
		if l == nil || len(l) == 0 {
			continue
		}
		for _, gl := range l {
			if gl.ID == id {
				log4go.Info("👌 ---> reset_geo : %s , %s , [ %v -> %v ] ", protoID, id.Pretty(), gl, location)
				gl.Latitude = location.Latitude
				gl.Longitude = location.Longitude
				break
			}
		}
	}
	/*	for _, protoID := range params.P_AGENT_ALL {
		l := self.table[protoID]
		if l == nil || l.Len() == 0 {
			continue
		}
		for e := l.Front(); e != nil; e = e.Next() {
			if gl, ok := e.Value.(*types.GeoLocation); ok && gl.ID == id {
				log4go.Info("---> reset_geo : %s , %s , [ %v -> %v ] ", protoID, id.Pretty(), gl, location)
				gl.Latitude = location.Latitude
				gl.Longitude = location.Longitude
				break
			} else if !ok {
				l.Remove(e)
			}
		}
	}*/
}

func (self *Astable) QuerySelfLocation(target peer.ID) error {
	if self.node.GetGeoLocation() == nil {
		req := agents_pb.NewMessage(agents_pb.AgentMessage_MY_LOCATION)
		resp, err := self.SendMsg(context.Background(), target, req)
		log4go.Info("<<QuerySelfLocation>> my_location_response : %v , %v", err, resp)
		if err != nil {
			log4go.Error("🛰️ 🌍 get_my_location error : %v", err)
			return err
		}
		if !tookit.VerifyLocation(resp.Location.Latitude, resp.Location.Longitude) {
			log4go.Error("🛰️ 🌍 get_my_location fail : %v", resp.Location)
			return errors.New("get_my_location_fail")
		}
		gl := types.NewGeoLocation(float64(resp.Location.Longitude), float64(resp.Location.Latitude))
		gl.ID = self.node.Host().ID()
		self.node.SetGeoLocation(gl)
	}
	return nil
}

func (self *Astable) loop() {
	var (
		timer        = time.NewTimer(time.Second * time.Duration(self.intrval))
		resetIntrval = func(t *time.Timer, ast *Astable) {
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
	)

	// fetch in peers
	doloop := func() {
		var (
			//best  peer.ID
			count = int32(0)
			wg    = new(sync.WaitGroup)
			// 需要发送 AgentMessage_COUNT_AS_TAB 消息的偏移量集合，随机产生，最多 25 个
			sendTask         = set.New()
			totalSendTaskLog = int32(0)
			st               = 25
			conns            = self.node.Host().Network().Conns()
		)

		if len(conns) < st {
			for i := 0; i < len(conns); i++ {
				sendTask.Insert(i)
			}
		} else {
			for i := 0; i < st; i++ {
				sendTask.Insert(rand.Intn(len(conns)))
			}
		}

		// 在邻居列表里寻找 as
		for i, conn := range conns {
			p := conn.RemotePeer()
			//log4go.Info("<<astabloop-conns>>  %d , %s", i, p)
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
							if self.QuerySelfLocation(p) == nil {
								log4go.Info("conns -> astab.Append : %s", gl.ID.Pretty())
								self.Append(pid, gl)
							}
						}
					default:
					}
				}
			}

			// 通过 countAsTabCache 和 isIgnoreBroadcast 双重拦截发送重复的消息
			if !self.isIgnoreBroadcast(p.Pretty()) && sendTask.Exists(i) {
				self.resetBestAndCount(p, &count, wg, &totalSendTaskLog)
			}
		}
		wg.Wait()
		log4go.Info("Agent : COUNT_AS_TAB : count=%d , best=%s , total_conns = %d , total_send = %d", count, self.best, len(conns), totalSendTaskLog)
		// 周期到邻居节点去获取 astab , 只去 best 节点获取
		self.getAstabFromBestPeer(&count)
	}

	for range timer.C {
		resetIntrval(timer, self)
		doloop()
	}
}

func (self *Astable) resetBestAndCount(p peer.ID, count *int32, wg *sync.WaitGroup, counter *int32) {
	catc, ok := self.countAsTabCache.Get(p)
	if ok {
		atr := catc.(*countAsTabRecord)
		if atr.Expired() {
			self.countAsTabCache.Remove(p)
		} else {
			return
		}
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		am := agents_pb.NewMessage(agents_pb.AgentMessage_COUNT_AS_TAB)
		respmsg, err := self.SendMsg(context.Background(), p, am)
		atomic.AddInt32(counter, 1)
		//log4go.Info("<<AgentMessage_COUNT_AS_TAB>> : err=%v, %v", err, p)
		if err != nil {
			self.appendIgnoreBroadcast(p.Pretty())
			return
		}
		self.countAsTabCache.Add(p, newCountAsTabRecord(p, respmsg.Count))
		if atomic.LoadInt32(count) <= respmsg.Count {
			atomic.StoreInt32(count, respmsg.Count)
			self.best = p
		}
	}()
}

// 周期到邻居节点去获取 astab , 只去 best 节点获取
func (self *Astable) getAstabFromBestPeer(count *int32) {
	best := self.best
	log4go.Info("<<astabloop-count>>  %d, %s", count, best)
	if atomic.LoadInt32(count) > 0 {
		if best.Pretty() == self.prebest.Pretty() && *count <= self.precount {
			return
		}
		self.precount, self.prebest = *count, best
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
				log4go.Debug("best -> astab.Append : %s", gl.ID)
				self.Append(pid, gl)
			}
		}
	}
}
