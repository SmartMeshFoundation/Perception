package core

import (
	"context"
	"github.com/SmartMeshFoundation/Perception/cmd/utils"
	"github.com/SmartMeshFoundation/Perception/params"
	inet "gx/ipfs/QmPtFaR7BWHLAjSwLh9kXcyrgTzDpuhcWLkx8ioa9RMYnx/go-libp2p-net"
	ma "gx/ipfs/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmSzEdVLaPMQGAKKGo4mKjsbWcfz6w8CoDjhRPxdk7xYdn/go-ipfs-addr"
	"gx/ipfs/QmZ9zH2FnLcxv1xyzFeUpDUeo55xEhZQHgveZijcxr7TLj/go-libp2p-peerstore"
	"sync"
	"sync/atomic"
	"time"
)

func loop_bootstrap(node *NodeImpl) {
	var wg sync.WaitGroup
	q, qw := make(chan int), int32(0)
	ctx := context.Background()
	timer := time.NewTimer(time.Millisecond * 1)
	defer timer.Stop()
	go func() {
		defer close(q)
		<-q
		node.Bootstrap(ctx)
		log4go.Info("-- 🌍 routing bootstrap -->")
		utils.Astab.Start()
	}()
	for range timer.C {
		timer.Reset(time.Second * params.BootstrapInterval)
		bootstrap(&wg, node)
		wg.Wait()
		if atomic.LoadInt32(&qw) == 0 && len(node.Host().Network().Peers()) >= 1 {
			atomic.AddInt32(&qw, 1)
			q <- 0
		}
	}

}

func bootstrap(wg *sync.WaitGroup, node *NodeImpl) {
	conns := node.Host().Network().Conns()
	if len(conns) < len(params.Bootnodes) {
		for _, bootnode := range params.Bootnodes {
			addr, err := ipfsaddr.ParseString(bootnode)
			if err != nil {
				continue
			}
			wg.Add(1)
			go func() {
				ctx := context.Background()
				ctx, cancel := context.WithTimeout(ctx, time.Second*5)
				defer func() {
					wg.Done()
					cancel()
				}()
				if node.Host().Network().Connectedness(addr.ID()) != inet.Connected {
					log4go.Debug("bootstrap_init -> %s", addr.ID().Pretty())
					if err := node.ConnectWithTTL(ctx, addr.ID(), []ma.Multiaddr{addr.Transport()}, peerstore.PermanentAddrTTL); err != nil {
						//log4go.Error("id:%v , transport:%v , err:%v", addr.ID().Pretty(), addr.Transport(), err)
					}
				}
			}()
		}
	}
}
