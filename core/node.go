package core

import (
	"context"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/agents"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	"github.com/SmartMeshFoundation/Perception/rpc"
	"github.com/SmartMeshFoundation/Perception/rpc/service"
	"gx/ipfs/QmfZaUn1SJEsSij84UGBhtqyN1J3UECdujwkZV1ve7rRWX/go-libp2p-kad-dht"
	opts "gx/ipfs/QmfZaUn1SJEsSij84UGBhtqyN1J3UECdujwkZV1ve7rRWX/go-libp2p-kad-dht/opts"
	//ic "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	circuit "gx/ipfs/QmNcNWuV38HBGYtRUi3okmfXSMEmXWwNgb82N3PzqqsHhY/go-libp2p-circuit"
	ic "gx/ipfs/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	inet "gx/ipfs/QmPtFaR7BWHLAjSwLh9kXcyrgTzDpuhcWLkx8ioa9RMYnx/go-libp2p-net"
	mafilter "gx/ipfs/QmQJRvWaYAvU3Mdtk33ADXr9JAZwKMBYBGPkRQBDvyj2nn/go-maddr-filter"
	ma "gx/ipfs/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmV281Yximj5ftHwYMSRCLbFhErRxqUFP2F8Rfa19LeToz/go-libp2p"
	"gx/ipfs/QmV281Yximj5ftHwYMSRCLbFhErRxqUFP2F8Rfa19LeToz/go-libp2p/p2p/discovery"
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
	host        host.Host
	Routing     *dht.IpfsDHT
	sg          types.StreamGenerater
	Discovery   discovery.Service
	agentServer types.AgentServer
	agentClient types.AgentClient
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
		LogOutput: os.Stdout,
		//LogOutput:              ioutil.Discard,
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

func NewNode(key ic.PrivKey, port int) *NodeImpl {
	ctx := context.Background()
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
	node := &NodeImpl{hostObj, d, nil, nil, nil, nil}
	sg := NewStreamGenerater(node)
	node.sg = sg
	return node
}

func (self *NodeImpl) Close() {
	self.Routing.Close()
	self.host.Close()
}

func (self *NodeImpl) Bootstrap(ctx context.Context) error {
	return self.Routing.Bootstrap(ctx)
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
func (self *NodeImpl) Connect(ctx context.Context, targetID interface{}, targetAddrs []ma.Multiaddr) error {
	return self.ConnectWithTTL(ctx, targetID, targetAddrs, peerstore.TempAddrTTL)
}

func (self *NodeImpl) PutValue(ctx context.Context, key string, value []byte) error {
	err := self.Routing.PutValue(ctx, key, value)
	if err != nil {
		log4go.Error("node.put_err -> err=%v , key=%s", err, key)
	}
	return err
}

func (self *NodeImpl) GetValue(ctx context.Context, key string) ([]byte, error) {
	buf, err := self.Routing.GetValue(ctx, key)
	if err != nil {
		log4go.Error("node.get_err -> err=%v , key=%s", err, key)
	}
	return buf, err
}

func (self *NodeImpl) FindPeer(ctx context.Context, targetID interface{}, findby chan peer.ID) (pstore.PeerInfo, error) {
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
		stop <- 0
	}()
	//ctx = notif.RegisterForQueryEvents(ctx, qch)
	ctx, qch := notif.RegisterForQueryEvents(ctx)
	go func() {
		var id peer.ID
		for {
			select {
			case q := <-qch:
				if q.Type == notif.PeerFindby {
					id = q.ID
					log4go.Info(" 🌟️   try to conn temp or relay by %s", id.Pretty())
				}
			case <-stop:
				if id != "" && findby != nil {
					findby <- id
				} else if findby != nil {
					findby <- ""
				}
				close(stop)
				log4go.Info("👋  👋  👋")
				return
			}
		}
	}()
	pi, err := self.Routing.FindPeer(ctx, tid)
	return pi, err
}

func (self *NodeImpl) Start(seed bool) {
	self.setupNetworkID()
	sh := NewStreamHandler(self)
	self.host.SetStreamHandler(params.P_CHANNEL_PING, sh.pingHandler)
	self.host.SetStreamHandler(params.P_CHANNEL_FILE, sh.fileHandler)
	self.host.SetStreamHandler(params.P_CHANNEL_BRIDGE, sh.bridgeHandler)
	self.host.SetStreamHandler(params.P_CHANNEL_AGENTS, sh.agentsHandler)

	service.Registry("funcs", "0.0.1", service.NewFuncs(self))
	rpc := rpc.NewRPCServer(self)
	go rpc.Start()
	if !seed {
		go loop_bootstrap(self)
	}
	if self.agentServer != nil {
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
}

func (self *NodeImpl) setupNetworkID() {

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
