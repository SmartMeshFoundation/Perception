package core

import (
	"context"
	"fmt"
	"github.com/cc14514/go-geoip2"
	"github.com/SmartMeshFoundation/Perception/agents"
	"github.com/SmartMeshFoundation/Perception/cmd/utils"
	"github.com/SmartMeshFoundation/Perception/core/discovery"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	"github.com/SmartMeshFoundation/Perception/rpc"
	"github.com/SmartMeshFoundation/Perception/rpc/service"
	"github.com/SmartMeshFoundation/Perception/tookit"
	"gx/ipfs/QmfZaUn1SJEsSij84UGBhtqyN1J3UECdujwkZV1ve7rRWX/go-libp2p-kad-dht"
	opts "gx/ipfs/QmfZaUn1SJEsSij84UGBhtqyN1J3UECdujwkZV1ve7rRWX/go-libp2p-kad-dht/opts"
	"io/ioutil"
	"net"

	circuit "gx/ipfs/QmNcNWuV38HBGYtRUi3okmfXSMEmXWwNgb82N3PzqqsHhY/go-libp2p-circuit"
	ic "gx/ipfs/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	inet "gx/ipfs/QmPtFaR7BWHLAjSwLh9kXcyrgTzDpuhcWLkx8ioa9RMYnx/go-libp2p-net"
	mafilter "gx/ipfs/QmQJRvWaYAvU3Mdtk33ADXr9JAZwKMBYBGPkRQBDvyj2nn/go-maddr-filter"
	ma "gx/ipfs/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmV281Yximj5ftHwYMSRCLbFhErRxqUFP2F8Rfa19LeToz/go-libp2p"
	p2pbhost "gx/ipfs/QmV281Yximj5ftHwYMSRCLbFhErRxqUFP2F8Rfa19LeToz/go-libp2p/p2p/host/basic"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	smux "gx/ipfs/QmY9JXR3FupnYAYJWK9aMr9bCpqWKcToQ1tz8DVGTrHpHw/go-stream-muxer"
	"gx/ipfs/QmZ9zH2FnLcxv1xyzFeUpDUeo55xEhZQHgveZijcxr7TLj/go-libp2p-peerstore"
	pstore "gx/ipfs/QmZ9zH2FnLcxv1xyzFeUpDUeo55xEhZQHgveZijcxr7TLj/go-libp2p-peerstore"
	"gx/ipfs/QmZNGvvTqtVtGzVxkhRCu5nuwbytdRdtHtJbWLGxeMYAkt/go-libp2p-connmgr"
	mplex "gx/ipfs/QmdiBZzwGtN2yHJrWD9ojQ7ASS48nv7BcojWLkYd1ZtrV2/go-smux-multiplex"
	yamux "gx/ipfs/Qmdps3CYh5htGQSrPvzg5PHouVexLmtpbuLCqc4vuej8PC/go-smux-yamux"
	notif "gx/ipfs/QmeViStrmu4NXFC7M7j4mUMr3FPbBeJdmT3mnbTFq2ZqKt/go-libp2p-routing/notifications"
	"gx/ipfs/QmfD51tKgJiTMnW9JEiDiPwsCY4mqUoxkhKhBfyW12spTC/go-libp2p-host"
	"os"
	"reflect"
	"strings"
	"time"
)

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

type NodeImpl struct {
	ctx         context.Context
	cancel      context.CancelFunc
	host        host.Host
	Routing     *dht.IpfsDHT
	sg          types.StreamGenerater
	Discovery   discovery.Service
	agentServer types.AgentServer
	agentClient types.AgentClient
	liveserver  types.LiveServer

	ipdb    *geoip2.DBReader
	selfgeo *types.GeoLocation
}

func (self *NodeImpl) SetGeoLocation(gl *types.GeoLocation) {
	log4go.Info("🛰️ 🌍 Set self location success : %v", gl)
	self.selfgeo = gl
}

func (self *NodeImpl) GetGeoipDB() *geoip2.DBReader {
	return self.ipdb
}

func (self *NodeImpl) GetGeoLocation() *types.GeoLocation {
	return self.selfgeo
}

func (self *NodeImpl) SetGeoipDB(db *geoip2.DBReader) {
	log4go.Info("agent-server : geo_ip_db started")
	self.ipdb = db
}

func (self *NodeImpl) SetLiveServer(ls types.LiveServer) {
	self.liveserver = ls
}

func (self *NodeImpl) GetLiveServer() types.LiveServer {
	return self.liveserver
}

func (self *NodeImpl) SetAgentClient(ac types.AgentClient) {
	self.agentClient = ac
}

func (self *NodeImpl) GetAgentClient() types.AgentClient {
	return self.agentClient
}

func (self *NodeImpl) GetAgentServer() types.AgentServer {
	return self.agentServer
}

func (self *NodeImpl) SetAgentServer(as types.AgentServer) {
	self.agentServer = as
}

func (self *NodeImpl) SetDiscovery(d discovery.Service) {
	d.RegisterNotifee(self)
	self.Discovery = d
}

func (self *NodeImpl) GetStreamGenerater() types.StreamGenerater {
	return self.sg
}

func (self *NodeImpl) Host() host.Host {
	return self.host
}

func makeSmuxTransportOption() libp2p.Option {
	const yamuxID = "/yamux/1.0.0"
	const mplexID = "/mplex/6.7.0"

	ymxtpt := &yamux.Transport{
		AcceptBacklog:          512,
		ConnectionWriteTimeout: time.Second * 10,
		KeepAliveInterval:      time.Second * 30,
		EnableKeepAlive:        true,
		MaxStreamWindowSize:    uint32(1024 * 512), // same ipfs
		//MaxStreamWindowSize:    uint32(1024 * 1024 * 512),
		//LogOutput: os.Stdout,
		LogOutput: ioutil.Discard,
	}

	if os.Getenv("YAMUX_DEBUG") != "" {
		ymxtpt.LogOutput = os.Stderr
	}

	muxers := map[string]smux.Transport{yamuxID: ymxtpt}
	muxers[mplexID] = mplex.DefaultTransport

	// Allow muxer preference order overriding
	order := []string{yamuxID, mplexID}
	if prefs := os.Getenv("LIBP2P_MUX_PREFS"); prefs != "" {
		order = strings.Fields(prefs)
	}

	opts := make([]libp2p.Option, 0, len(order))
	for _, id := range order {
		tpt, ok := muxers[id]
		if !ok {
			log4go.Warn("unknown or duplicate muxer in LIBP2P_MUX_PREFS: %s", id)
			continue
		}
		delete(muxers, id)
		opts = append(opts, libp2p.Muxer(id, tpt))
	}

	return libp2p.ChainOptions(opts...)
}

func libp2popts(ctx context.Context, key ic.PrivKey, port int) []libp2p.Option {
	var libp2pOpts []libp2p.Option

	// set pk
	libp2pOpts = append(libp2pOpts, libp2p.Identity(key))

	// listen address
	libp2pOpts = append(libp2pOpts, libp2p.ListenAddrStrings(
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port),
		fmt.Sprintf("/ip6/::/tcp/%d", port),
	))

	composeAddrsFactory := func(f, g p2pbhost.AddrsFactory) p2pbhost.AddrsFactory {
		return func(addrs []ma.Multiaddr) []ma.Multiaddr {
			return f(g(addrs))
		}
	}
	filters := mafilter.NewFilters()
	af := func(addrs []ma.Multiaddr) []ma.Multiaddr {
		var out []ma.Multiaddr
		for _, maddr := range addrs {
			// check for /ipcidr matches
			if !filters.AddrBlocked(maddr) {
				out = append(out, maddr)
			}
		}
		return out
	}
	af = composeAddrsFactory(af, func(addrs []ma.Multiaddr) []ma.Multiaddr {
		var raddrs []ma.Multiaddr
		for _, addr := range addrs {
			_, err := addr.ValueForProtocol(circuit.P_CIRCUIT)
			if err == nil {
				continue
			}
			raddrs = append(raddrs, addr)
		}
		//fmt.Println(addrs," ==> ",raddrs)
		return raddrs
	})

	//libp2pOpts = append(libp2pOpts, libp2p.AddrsFactory(af))

	// 支持 upnp
	libp2pOpts = append(libp2pOpts, libp2p.NATPortMap())

	libp2pOpts = append(libp2pOpts, libp2p.ConnectionManager(connmgr.NewConnManager(600, 900, time.Second*20)))

	/*
		if false {
			libp2pOpts = append(libp2pOpts, libp2p.AddrsFactory(af))
		}*/
	libp2pOpts = append(libp2pOpts, libp2p.AddrsFactory(af))

	// disable
	if 1 == 2 {
		/*		// TODO local db peerstore
				dataPath := path.Join(params.HomeDir, "nodes")
				dopt := badger.DefaultOptions
				dopt.ValueLogLoadingMode = options.FileIO
				ds, err := badger.NewDatastore(dataPath, &dopt)
				if err == nil {
					psopt := pstoreds.DefaultOpts()
					psopt.CacheSize = 512
					//psopt.TTLInterval = time.Second * 3
					//psopt.WriteRetries = 3
					if ps, err := pstoreds.NewPeerstore(ctx, ds, psopt); err == nil {
						libp2pOpts = append(libp2pOpts, libp2p.Peerstore(ps))
					}
				}*/
		// TODO 支持 relay
		libp2pOpts = append(libp2pOpts, libp2p.EnableRelay(circuit.OptHop))
	}

	libp2pOpts = append(libp2pOpts, makeSmuxTransportOption())

	// set networkid to build a private net
	if p, err := params.NewProtector(); err == nil {
		log4go.Info("🔒 pnet started .")
		libp2pOpts = append(libp2pOpts, libp2p.PrivateNetwork(p))
	}

	return libp2pOpts
}

func NewNode(ctx context.Context, cancel context.CancelFunc, key ic.PrivKey, port int) *NodeImpl {
	hostObj, err := libp2p.New(ctx, libp2popts(ctx, key, port)...)
	if err != nil {
		panic(err)
	}

	la := hostObj.Network().ListenAddresses()
	hostObj.Network().Peerstore().AddAddrs(hostObj.ID(), la, peerstore.ProviderAddrTTL)
	d, err := dht.New(ctx, hostObj, opts.NamespacedValidator("cc14514", blankValidator{}))
	if err != nil {
		panic(err)
	}
	node := &NodeImpl{ctx: ctx, cancel: cancel, host: hostObj, Routing: d}
	sg := NewStreamGenerater(node)
	node.sg = sg
	return node
}

func (self *NodeImpl) Close() {
	self.Routing.Close()
	self.host.Close()
	if self.cancel != nil {
		self.cancel()
	}

	select {
	case _, ok := <-utils.Stop:
		if !ok {
			fmt.Println("normal_skip")
			return
		}
	case <-time.After(100 * time.Millisecond):
		fmt.Println("close_node.")
		close(utils.Stop)
		return
	}

}

func (self *NodeImpl) Bootstrap() error {
	return self.Routing.Bootstrap(self.ctx)
}

func (self *NodeImpl) ConnectWithTTL(ctx context.Context, targetID interface{}, targetAddrs []ma.Multiaddr, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()
	var tid peer.ID
	if "string" == reflect.TypeOf(targetID).Name() {
		stid := targetID.(string)
		tid, _ = peer.IDFromString(stid)
	} else {
		tid = targetID.(peer.ID)
	}
	a := self.host
	a.Peerstore().AddAddrs(tid, targetAddrs, ttl)
	pi := peerstore.PeerInfo{ID: tid}
	if err := a.Connect(ctx, pi); err != nil {
		return err
	}

	return nil
}
func (self *NodeImpl) Connect(targetID interface{}, targetAddrs []ma.Multiaddr) error {
	return self.ConnectWithTTL(self.ctx, targetID, targetAddrs, peerstore.TempAddrTTL)
}

func (self *NodeImpl) PutValue(key string, value []byte) error {
	err := self.Routing.PutValue(self.ctx, key, value)
	if err != nil {
		log4go.Error("node.put_err -> err=%v , key=%s", err, key)
	}
	return err
}

func (self *NodeImpl) GetValue(key string) ([]byte, error) {
	buf, err := self.Routing.GetValue(self.ctx, key)
	if err != nil {
		log4go.Error("node.get_err -> err=%v , key=%s", err, key)
	}
	return buf, err
}

func (self *NodeImpl) FindPeer(ctx context.Context, targetID interface{}, findby chan peer.ID) (pstore.PeerInfo, error) {
	if ctx == nil {
		ctx = self.ctx
	}
	var (
		tid  peer.ID
		err  error
		stop = make(chan int)
		//qch, stop = make(chan *notif.QueryEvent), make(chan int)
	)
	if "string" == reflect.TypeOf(targetID).Name() {
		stid := targetID.(string)
		tid, err = peer.IDB58Decode(stid)
		if err != nil {
			log4go.Error(err)
			return pstore.PeerInfo{}, err
		}
	} else {
		tid = targetID.(peer.ID)
	}
	defer func() {
		close(stop)
	}()
	//ctx = notif.RegisterForQueryEvents(ctx, qch)
	ctx, qch := notif.RegisterForQueryEvents(ctx)
	go func() {
		var id peer.ID
		for {
			select {
			case q := <-qch:
				if q == nil {
					log4go.Debug("<<QueryEvents>> return_nil")
					return
				}
				if q.Type == notif.PeerFindby {
					id = q.ID
					log4go.Info("try to conn temp or relay by %s", id)
				}
			case <-stop:
				if id != "" && findby != nil {
					findby <- id
				} else if findby != nil {
					findby <- ""
				}
				return
			}
		}
	}()
	pi, err := self.Routing.FindPeer(ctx, tid)
	return pi, err
}

func (self *NodeImpl) Context() context.Context {
	return self.ctx
}

func (self *NodeImpl) Start(nd bool) {
	sh := NewStreamHandler(self)
	self.host.SetStreamHandler(params.P_CHANNEL_PING, sh.pingHandler)
	self.host.SetStreamHandler(params.P_CHANNEL_LIVE, sh.liveHandler)
	self.host.SetStreamHandler(params.P_CHANNEL_FILE, sh.fileHandler)
	self.host.SetStreamHandler(params.P_CHANNEL_BRIDGE, sh.bridgeHandler)
	self.host.SetStreamHandler(params.P_CHANNEL_AGENTS, sh.agentsHandler)

	service.Registry("funcs", "0.0.1", service.NewFuncs(self))
	rpc := rpc.NewRPCServer(self)
	go rpc.Start(self.ctx)
	if !nd {
		go loop_bootstrap(self)
	}
	if self.agentServer != nil {
		self.setupLocation()
		if agents.Web3RpcAgentConfig != "" {
			self.Host().SetStreamHandler(params.P_AGENT_WEB3_RPC, sh.agentsWeb3rpcHandler)
		}
		if agents.Web3WsAgentConfig != "" {
			self.Host().SetStreamHandler(params.P_AGENT_WEB3_WS, sh.agentsWeb3wsHandler)
		}
		if agents.IpfsApiAgentConfig != "" {
			self.Host().SetStreamHandler(params.P_AGENT_IPFS_API, sh.agentsIpfsapiHandler)
		}
		if agents.IpfsGatewayAgentConfig != "" {
			self.Host().SetStreamHandler(params.P_AGENT_IPFS_GATEWAY, sh.agentsIpfsgatewayHandler)
		}
		if agents.RestAgentConfig != "" {
			self.Host().SetStreamHandler(params.P_AGENT_REST, sh.agentsRestHandler)
		}
		self.agentServer.Start()
	} else if self.agentClient != nil {
		self.agentClient.Start()
	}
	if self.liveserver != nil {
		self.liveserver.Start()
	}
}

func (self *NodeImpl) GetIP4AddrByMultiaddr(addrs []ma.Multiaddr) []string {
	ips := make([]string, 0)
	for _, ma := range addrs {
		maar := strings.Split(strings.Trim(ma.String(), " "), "/")
		if len(maar) > 0 && (maar[0] == "ip4" || maar[1] == "ip4") {
			switch maar[1] {
			case "ip4":
				ips = append(ips, maar[2])
			default:
				ips = append(ips, maar[1])
			}
		}
	}
	return ips
}

func (self *NodeImpl) setupLocation() {
	if self.ipdb == nil {
		log4go.Error("ignor setupNetwork , geoipdb not started ...")
		return
	}
	go func() {
		log4go.Info("geoipdb already started , wait for get self geo ...")
		var ips []string
		for i := 0; i < 3; i++ {
			mas := self.Host().Addrs()
			ips = self.GetIP4AddrByMultiaddr(mas)
			for _, ip := range ips {
				c, err := self.ipdb.City(net.ParseIP(ip))
				// empty city name is private ip addr
				if len(c.City.Names) == 0 {
					c.Location.Longitude, c.Location.Latitude = -203, -203
				}
				if err == nil && tookit.VerifyLocation(c.Location.Latitude, c.Location.Longitude) {
					self.selfgeo = types.NewGeoLocation(c.Location.Longitude, c.Location.Latitude)
					h, _ := tookit.GeoEncode(self.selfgeo.Latitude, self.selfgeo.Longitude, params.GeoPrecision)
					self.selfgeo.Geohash = h
					self.selfgeo.ID = self.host.ID()
					log4go.Info("get self geo success , location : %v", *self.selfgeo)
					return
				}
			}
			select {
			case <-self.ctx.Done():
				return
			case <-time.After(2 * time.Second):
				continue
			}
		}
		// 如果超过 5 秒没拿到，也许是本机没有直接公网ip，需要问邻居节点要一个了
		log4go.Warn("public ip not found", ips)
		for {
			log4go.Warn("try ask astab by async action.")
			params.AACh <- params.NewAA(params.AA_GET_MY_LOCATION, nil)
			select {
			case <-self.ctx.Done():
				return
			case <-time.After(3 * time.Second):
				continue
			}
			if self.selfgeo != nil && tookit.VerifyLocation(self.selfgeo.Latitude, self.selfgeo.Longitude) {
				log4go.Info("query self geo success , location : %v", *self.selfgeo)
				return
			}
		}
	}()
}
func (self *NodeImpl) HandlePeerFound(p pstore.PeerInfo) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	if self.Host().Network().Connectedness(p.ID) != inet.Connected {
		if err := self.Host().Connect(ctx, p); err == nil {
			//TODO 黑名单 , 防止反复连
			//log4go.Warn(" 🌾 mdns : connected -> %s : %v", p.ID.Pretty(), p.Addrs)
		}
	}
}
