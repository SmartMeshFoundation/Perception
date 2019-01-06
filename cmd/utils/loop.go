package utils

import (
	"context"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"sync/atomic"
)

var (
	aa_get_location = int32(0)
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

func handler(aa params.AsyncAction, node types.Node) {
	switch aa {
	case params.AA_GET_LOCATION:
		if atomic.LoadInt32(&aa_get_location) == 1 {
			log4go.Warn("refuse : already_handler_get_location_action.")
			return
		}
		atomic.StoreInt32(&aa_get_location, 1)
		defer atomic.StoreInt32(&aa_get_location, 0)
		log4go.Info("loop_handler_get_location_action")
		if astab := Astab.GetTable(); astab != nil {
			for p, l := range astab {
				log4go.Info("do_handler : %s --> %d", p, l.Len())
				if l.Len() > 0 {
					for e := l.Front(); e != nil; e = e.Next() {
						as, ok := e.Value.(*types.GeoLocation)
						if !ok {
							l.Remove(e)
							continue
						}
						targetID := as.ID
						log4go.Info("do_handler : sent_my_localtion_message --> %s", targetID)
					}
				}
			}
		}
	}
}
