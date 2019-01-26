package discovery

import (
	"context"
	"errors"
	"gx/ipfs/QmQVUtnrNGtCRkCMpXgpApfzQjc8FDaDVxHqWH8cnZQeh5/go-multiaddr-net"
	ma "gx/ipfs/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	pstore "gx/ipfs/QmZ9zH2FnLcxv1xyzFeUpDUeo55xEhZQHgveZijcxr7TLj/go-libp2p-peerstore"
	//logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
	"gx/ipfs/QmekaTKpWkYGcn4ZEC5PwJDRCQHapwugmmG86g2Xpz5GBH/mdns"
	"gx/ipfs/QmfD51tKgJiTMnW9JEiDiPwsCY4mqUoxkhKhBfyW12spTC/go-libp2p-host"
	"io"
	"net"
	"sync"
	"time"
)

func init() {
	// don't let mdns use logging...
	mdns.DisableLogging = true
}

//var log = logging.Logger("mdns")

const ServiceTag = "_ipfs-discovery._udp"

type Service interface {
	io.Closer
	RegisterNotifee(Notifee)
	UnregisterNotifee(Notifee)
}

type Notifee interface {
	HandlePeerFound(pstore.PeerInfo)
}

type mdnsService struct {
	server  *mdns.Server
	service *mdns.MDNSService
	host    host.Host
	tag     string

	lk       sync.Mutex
	notifees []Notifee
	interval time.Duration
}

func getDialableListenAddrs(ph host.Host) ([]*net.TCPAddr, error) {
	var out []*net.TCPAddr
	for _, addr := range ph.Addrs() {
		na, err := manet.ToNetAddr(addr)
		if err != nil {
			continue
		}
		tcp, ok := na.(*net.TCPAddr)
		if ok {
			out = append(out, tcp)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("failed to find good external addr from peerhost")
	}
	return out, nil
}

func NewMdnsService(ctx context.Context, peerhost host.Host, interval time.Duration, serviceTag string) (Service, error) {

	var ipaddrs []net.IP
	port := 4001

	addrs, err := getDialableListenAddrs(peerhost)
	log4go.Info("<<mdns>> 1, err=%v", err)
	if err != nil {
		log4go.Error(err)
	} else {
		port = addrs[0].Port
		for _, a := range addrs {
			ipaddrs = append(ipaddrs, a.IP)
		}
	}
	myid := peerhost.ID().Pretty()
	log4go.Info("<<mdns>> 2, myid=%s", myid)

	info := []string{myid}
	if serviceTag == "" {
		serviceTag = ServiceTag
	}
	service, err := mdns.NewMDNSService(myid, serviceTag, "", "", port, ipaddrs, info)
	log4go.Info("<<mdns>> 3, err=%v", err)
	if err != nil {
		return nil, err
	}

	// Create the mDNS server, defer shutdown
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	log4go.Info("<<mdns>> 4, err=%v", err)
	if err != nil {
		return nil, err
	}

	s := &mdnsService{
		server:   server,
		service:  service,
		host:     peerhost,
		interval: interval,
		tag:      serviceTag,
	}

	go s.pollForEntries(ctx)
	log4go.Info("<<mdns>> 5, err=%v", err)
	return s, nil
}

func (m *mdnsService) Close() error {
	return m.server.Shutdown()
}

func (m *mdnsService) pollForEntries(ctx context.Context) {

	ticker := time.NewTicker(m.interval)
	for {
		//execute mdns query right away at method call and then with every tick
		entriesCh := make(chan *mdns.ServiceEntry, 16)
		go func() {
			for entry := range entriesCh {
				m.handleEntry(entry)
			}
		}()

		log4go.Debug("starting mdns query")
		qp := &mdns.QueryParam{
			Domain:  "local",
			Entries: entriesCh,
			Service: m.tag,
			Timeout: time.Second * 5,
		}

		err := mdns.Query(qp)
		if err != nil {
			log4go.Error("mdns lookup error: ", err)
		}
		close(entriesCh)
		log4go.Debug("mdns query complete")

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			log4go.Debug("mdns service halting")
			return
		}
	}
}

func (m *mdnsService) handleEntry(e *mdns.ServiceEntry) {
	log4go.Debug("Handling MDNS entry: %s:%d %s", e.AddrV4, e.Port, e.Info)
	mpeer, err := peer.IDB58Decode(e.Info)
	if err != nil {
		log4go.Debug("Error parsing peer ID from mdns entry: %s", err)
		return
	}

	if mpeer == m.host.ID() {
		log4go.Debug("got our own mdns entry, skipping")
		return
	}

	maddr, err := manet.FromNetAddr(&net.TCPAddr{
		IP:   e.AddrV4,
		Port: e.Port,
	})
	if err != nil {
		log4go.Debug("Error parsing multiaddr from mdns entry: %s", err)
		return
	}

	pi := pstore.PeerInfo{
		ID:    mpeer,
		Addrs: []ma.Multiaddr{maddr},
	}

	m.lk.Lock()
	for _, n := range m.notifees {
		go n.HandlePeerFound(pi)
	}
	m.lk.Unlock()
}

func (m *mdnsService) RegisterNotifee(n Notifee) {
	m.lk.Lock()
	m.notifees = append(m.notifees, n)
	m.lk.Unlock()
}

func (m *mdnsService) UnregisterNotifee(n Notifee) {
	m.lk.Lock()
	found := -1
	for i, notif := range m.notifees {
		if notif == n {
			found = i
			break
		}
	}
	if found != -1 {
		m.notifees = append(m.notifees[:found], m.notifees[found+1:]...)
	}
	m.lk.Unlock()
}
