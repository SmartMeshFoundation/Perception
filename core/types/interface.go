package types

import (
	"context"
	"github.com/cc14514/go-geoip2"
	"github.com/SmartMeshFoundation/Perception/core/discovery"
	inet "gx/ipfs/QmPtFaR7BWHLAjSwLh9kXcyrgTzDpuhcWLkx8ioa9RMYnx/go-libp2p-net"
	ma "gx/ipfs/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	pstore "gx/ipfs/QmZ9zH2FnLcxv1xyzFeUpDUeo55xEhZQHgveZijcxr7TLj/go-libp2p-peerstore"
	"gx/ipfs/QmfD51tKgJiTMnW9JEiDiPwsCY4mqUoxkhKhBfyW12spTC/go-libp2p-host"
	"net/http"
)

type GeoLocation struct {
	Longitude, Latitude float64
	Geohash             string
	ID                  peer.ID
}

func NewGeoLocation(lng, lat float64) *GeoLocation {
	g := &GeoLocation{lng, lat, "", peer.ID("")}
	return g
}

type Node interface {
	discovery.Notifee
	SetDiscovery(discovery.Service)
	SetAgentClient(AgentClient)
	SetGeoipDB(reader *geoip2.DBReader)
	GetGeoipDB() *geoip2.DBReader
	GetAgentClient() AgentClient
	SetGeoLocation(*GeoLocation)
	GetGeoLocation() *GeoLocation
	SetAgentServer(AgentServer)
	GetAgentServer() AgentServer
	Close()
	Host() host.Host
	Bootstrap() error
	Connect(targetID interface{}, targetAddrs []ma.Multiaddr) error
	PutValue(key string, value []byte) error
	GetValue(key string) ([]byte, error)
	FindPeer(ctx context.Context, targetID interface{}, findby chan peer.ID) (pstore.PeerInfo, error)
	Start(seed bool)
	GetStreamGenerater() StreamGenerater
	SetLiveServer(LiveServer)
	GetLiveServer() LiveServer
	GetIP4AddrByMultiaddr(addrs []ma.Multiaddr) []string
	Context() context.Context
}

type StreamGenerater interface {
	GenPingStream(targetid string) (inet.Stream, error)
	GenFileStream(filepath, targetid string, rangeFrom, rangeTo int64) (*FileStreamEntity, error)
	GenBridge(bid, tid string, action byte) (inet.Stream, error)
}

type FileStreamEntity struct {
	S        inet.Stream
	Size     int64  // filesize
	Hash     string // md5
	PartHash string // md5
}

type AgentCfg interface {
	Open() (name string, port, timeout int, err error)
}

type AgentServer interface {
	Start()
	RestAgent(c inet.Stream, cfg AgentCfg)
	IpfsAgent(c inet.Stream, cfg AgentCfg)
	Web3Agent(c inet.Stream, cfg AgentCfg)
	SetupReport(AgentCfg) error
	FetchReport(k, startDate, endDate string) (map[string]interface{}, error)
}

type AgentClient interface {
	Start()
	IpfsAgent(cfg AgentCfg)
	Web3Agent(cfg AgentCfg)
}

// rtmp server
type LiveServer interface {
	Start() error
	Stop() error
	GetControlHandler(pattern string) (HttpHandlerFn, error)
}

type HttpHandlerFn func(http.ResponseWriter, *http.Request)
