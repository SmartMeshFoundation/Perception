package agents

import (
	"fmt"
	"github.com/alecthomas/log4go"
	"github.com/SmartMeshFoundation/Perception/ldb"
	"github.com/SmartMeshFoundation/Perception/params"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	"math/big"
	"path"
	"sync"
	"time"
)

// agent-server upstream report
var (
	isEnable = make(map[string]bool)
	dbpath   = params.DataDir
	db       *ldb.LDBDatabase
)

func do_init() {
	var err error
	p := path.Join(params.DataDir, "report")
	db, err = ldb.NewLDBDatabase(p, 16, 16)
	log4go.Info("db_init : %s, err=%v", p, err)
	if err != nil {
		panic(err)
	}
}

func enableReport(k string) {
	o := &sync.Once{}
	go func(o *sync.Once) {
		o.Do(func() {
			do_init()
		})
	}(o)

	isEnable[k] = true
	log4go.Info("📒 enable report -> %s", k)
}

// 记录上行流量, k = ipfs / others...
func Record(name string, size *big.Int) {
	k := name
	switch protocol.ID(name) {
	case params.P_AGENT_IPFS_API, params.P_AGENT_IPFS_GATEWAY:
		k = "ipfs"
	}
	if !isEnable[k] {
		return
	}
	suffix := time.Now().Format("20060102")
	k = path.Join(k, suffix)
	v, err := db.Get([]byte(k))
	if err != nil {
		v = size.Bytes()
	} else {
		i := new(big.Int)
		i = i.SetBytes(v)
		v = i.Add(i, size).Bytes()
	}
	db.Put([]byte(k), v)
}

// k="ipfs" s="yyyymmdd" e="yyyymmdd"
// result key = "yyyymmdd"
func Report(k, s, e string) map[string]interface{} {
	if !isEnable[k] {
		return nil
	}
	result := make(map[string]interface{})
	start, _ := new(big.Int).SetString(s, 10)
	end, _ := new(big.Int).SetString(e, 10)
	if start.Cmp(end) > 0 {
		end = start
	}
	for ; start.Cmp(end) <= 0; start = new(big.Int).Add(start, big.NewInt(1)) {
		rk := start.String()
		kk := path.Join(k, rk)
		v, err := db.Get([]byte(kk))
		if err != nil {
			result[rk] = 0
		} else {
			result[rk] = new(big.Int).SetBytes(v).Int64()
		}
	}
	fmt.Println(result)
	return result
}
