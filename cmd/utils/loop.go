package utils

import (
	"context"
	"github.com/SmartMeshFoundation/Perception/agents/pb"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	"github.com/SmartMeshFoundation/Perception/tookit"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"sync"
	"sync/atomic"
	"time"
)

var (
	aa_get_my_location = int32(0)
	aa_get_as_location sync.Map
)

func AsyncActionLoop(ctx context.Context, node types.Node) {
	for {
		select {
		case aa := <-params.AACh:
			go handler(aa, node)
		case <-ctx.Done():
			return
		}
	}
}

func handler(aa *params.AA, node types.Node) {
	switch aa.Action {
	case params.AA_GET_AS_LOCATION:
		p, ok := aa.Payload.(peer.ID)
		if !ok {
			return
		}
		if node.Host().ID() == p {
			log4go.Debug("<<AA_GET_AS_LOCATION>> skip_self = %v", aa.Payload)
			return
		}
		_, duplicate := aa_get_as_location.LoadOrStore(p, 0)
		if duplicate {
			log4go.Debug("<<AA_GET_AS_LOCATION>> skip_duplicate = %v", aa.Payload)
			return
		}
		log4go.Debug("<<AA_GET_AS_LOCATION>> accept = %v", aa.Payload)
		ctx := context.Background()
		for i := 1; i < 3; i++ {
			<-time.After(time.Second * 2 * time.Duration(i))
			req := agents_pb.NewMessage(agents_pb.AgentMessage_YOUR_LOCATION)
			resp, err := Astab.SendMsg(ctx, p, req)
			log4go.Debug("<<AA_GET_AS_LOCATION>> result = %d , %v , %v", i, err, resp)
			if err == nil && tookit.VerifyLocation(resp.Location.Latitude, resp.Location.Longitude) {
				defer aa_get_as_location.Delete(p)
				gl := types.NewGeoLocation(float64(resp.Location.Longitude), float64(resp.Location.Latitude))
				gl.ID = p
				Astab.Reset(gl)
				return
			}
		}
	case params.AA_GET_MY_LOCATION:
		if atomic.LoadInt32(&aa_get_my_location) == 1 {
			log4go.Warn("refuse : already_handler_get_location_action.")
			return
		}
		atomic.StoreInt32(&aa_get_my_location, 1)
		defer atomic.StoreInt32(&aa_get_my_location, 0)
		log4go.Info("loop_handler_get_location_action")
		for p, _ := range astabToPeerMap(node.Host().ID()) {
			err := Astab.QuerySelfLocation(p)
			log4go.Info("do_handler : sent_my_localtion_message --> %s, err=%v", p, err)
			if err == nil {
				return
			}
		}
	}
}

func astabToPeerMap(selfid peer.ID) map[peer.ID]struct{} {
	r := make(map[peer.ID]struct{})
	if astab := Astab.GetTable(); astab != nil {
		for _, l := range astab {
			for _, as := range l {
				if as.ID == selfid {
					log4go.Info("do_handler_skip_self : astabToPeerMap")
					continue
				}
				if as.ID == "" {
					log4go.Info("do_handler_skip_nil : astabToPeerMap")
					continue
				}
				r[as.ID] = struct{}{}
			}
		}
	}
	return r
}
