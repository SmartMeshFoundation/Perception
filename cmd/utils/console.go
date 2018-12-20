package utils

import (
	"errors"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/params"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmYaVXmXZNpWs6owQ1rk5VAiwNinkTh2cYZuYx1JDSactL/go-lightrpc/rpcserver"
	"net/http"
)

var (
	client         = &http.Client{}
	funcs_myid_url = func() string {
		return fmt.Sprintf(`http://localhost:%d/api/?body={"sn":"sn-100","service":"funcs","method":"myid","params":{}}`, params.HTTPPort)
	}
	funcs_myaddrs_url = func() string {
		return fmt.Sprintf(`http://localhost:%d/api/?body={"sn":"sn-100","service":"funcs","method":"myaddrs","params":{}}`, params.HTTPPort)
	}
	funcs_conns_url = func() string {
		return fmt.Sprintf(`http://localhost:%d/api/?body={"sn":"sn-100","service":"funcs","method":"conns","params":{}}`, params.HTTPPort)
	}
	funcs_peers_url = func() string {
		return fmt.Sprintf(`http://localhost:%d/api/?body={"sn":"sn-100","service":"funcs","method":"peers","params":{}}`, params.HTTPPort)
	}
	funcs_conn_url = func(addr string) string {
		return fmt.Sprintf(`http://localhost:%d/api/?body={"sn":"sn-101","service":"funcs","method":"conn","params":{"addr":"%s"}}`, params.HTTPPort, addr)
	}
	/*
		funcs_bootstrap_url = func() string {
			return fmt.Sprintf(`http://localhost:%d/api/?body={"sn":"sn-102","service":"funcs","method":"bootstrap","params":{}}`,params.HTTPPort)
		}*/
	funcs_put_url = func(key, value string) string {
		return fmt.Sprintf(`http://localhost:%d/api/?body={"sn":"sn-101","service":"funcs","method":"put","params":{"key":"%s","value":"%s"}}`, params.HTTPPort, key, value)
	}
	funcs_local_url = func(id string) string {
		return fmt.Sprintf(`http://localhost:%d/api/?body={"sn":"sn-101","service":"funcs","method":"local","params":{"id":"%s"}}`, params.HTTPPort, id)
	}
	funcs_get_url = func(key string) string {
		return fmt.Sprintf(`http://localhost:%d/api/?body={"sn":"sn-101","service":"funcs","method":"get","params":{"key":"%s"}}`, params.HTTPPort, key)
	}
	funcs_bing_url = func(bo, to string) string {
		return fmt.Sprintf(`http://localhost:%d/api/?body={"sn":"sn-101","service":"funcs","method":"bing","params":{"bo":"%s","to":"%s"}}`, params.HTTPPort, bo, to)
	}
	funcs_ping_url = func(to string) string {
		return fmt.Sprintf(`http://localhost:%d/api/?body={"sn":"sn-101","service":"funcs","method":"ping","params":{"to":"%s"}}`, params.HTTPPort, to)
	}
	funcs_findpeer_url = func(to string) string {
		return fmt.Sprintf(`http://localhost:%d/api/?body={"sn":"sn-101","service":"funcs","method":"findpeer","params":{"to":"%s"}}`, params.HTTPPort, to)
	}
	funcs_getastab_url = func(id string) string {
		return fmt.Sprintf(`http://localhost:%d/api/?body={"sn":"sn-101","service":"funcs","method":"getastab","params":{"id":"%s"}}`, params.HTTPPort, id)
	}
	funcs_getfile_url = func(filepath, targetid, output string) string {
		return fmt.Sprintf(`http://localhost:%d/api/?body={"sn":"sn-101","service":"funcs","method":"getfile","params":{"filepath":"%s","targetid":"%s","output":"%s"}}`, params.HTTPPort, filepath, targetid, output)
	}
)

var Funcs = map[string]func(args ...string) (interface{}, error){
	"help": func(args ...string) (interface{}, error) {
		s := `
myid		local peer.ID	
myaddrs		listeners addrs 
peers				show peers 
findpeer <id>			findpeer by peer.ID
put <key> <value> 		put key value to dht
get <key>			get value by key from dht
bing <bridgeID> <id>	from bridgeID to target peer.ID send ping request , success return pong 	
ping <id>			ping peer.ID , success return pong 	
local <id>			local storage peer.ID address
getastab <id>		get agent-server table from peer.ID address
conn <addr>			connect to addr , "/ip4/[ip]/tcp/[port]/ipfs/[pid]"	
getfile <id> <filepath> <output>	id is peer.ID, filepath is peer.ID's file full path, output is local path
`

		/*
			scp <pid> <filepath>		copy <filepath> to pidNode's datadir/files for test transfer
			relay <fromID> <toID>	generate a channel for relay
		*/

		fmt.Println(s)
		return nil, nil
	},
	"myid": func(args ...string) (interface{}, error) {
		reqest, err := http.NewRequest("GET", funcs_myid_url(), nil)
		if err != nil {
			log4go.Error(err)
			return nil, err
		}
		response, _ := client.Do(reqest)
		if response != nil && response.Body != nil {
			defer response.Body.Close()
		}
		success := rpcserver.SuccessFromReader(response.Body)
		if success != nil && success.Success {
			myid := success.Entity
			return myid, nil
		}
		fmt.Println("not found .")
		return nil, nil
	},
	"myaddrs": func(args ...string) (interface{}, error) {
		reqest, err := http.NewRequest("GET", funcs_myaddrs_url(), nil)
		if err != nil {
			log4go.Error(err)
			return nil, err
		}
		response, _ := client.Do(reqest)
		if response != nil && response.Body != nil {
			defer response.Body.Close()
		}
		success := rpcserver.SuccessFromReader(response.Body)
		if success != nil && success.Success {
			myaddrs := success.Entity.(map[string]interface{})
			return myaddrs, nil
		}
		fmt.Println("not found .")
		return nil, nil
	},
	"conns": func(args ...string) (interface{}, error) {
		reqest, err := http.NewRequest("GET", funcs_conns_url(), nil)
		if err != nil {
			log4go.Error(err)
			return nil, err
		}
		response, _ := client.Do(reqest)
		if response != nil && response.Body != nil {
			defer response.Body.Close()
		}
		success := rpcserver.SuccessFromReader(response.Body)
		if success != nil && success.Success {
			m := success.Entity.(map[string]interface{})
			if c := m["Count"].(float64); c > 0 {
				for i, p := range m["Peers"].([]interface{}) {
					fmt.Println(i, "->", p.(string))
				}
				return nil, nil
			}
		}
		fmt.Println("not found connected .")
		return nil, nil
	},
	"peers": func(args ...string) (interface{}, error) {
		reqest, err := http.NewRequest("GET", funcs_peers_url(), nil)
		if err != nil {
			log4go.Error(err)
			return nil, err
		}
		response, _ := client.Do(reqest)
		if response != nil && response.Body != nil {
			defer response.Body.Close()
		}
		success := rpcserver.SuccessFromReader(response.Body)
		if success != nil && success.Success {
			m := success.Entity.(map[string]interface{})
			if c := m["Count"].(float64); c > 0 {
				for i, p := range m["Peers"].([]interface{}) {
					fmt.Println(i, "->", p.(string))
				}
				return nil, nil
			}
		}
		fmt.Println("peer not found .")
		return nil, nil
	},
	"conn": func(addrs ...string) (interface{}, error) {
		if len(addrs) > 0 {
			reqest, err := http.NewRequest("GET", funcs_conn_url(addrs[0]), nil)
			if err != nil {
				log4go.Error(err)
				return nil, err
			}
			response, _ := client.Do(reqest)
			if response != nil && response.Body != nil {
				defer response.Body.Close()
			}
			success := rpcserver.SuccessFromReader(response.Body)
			if success != nil && success.Success {
				fmt.Println("connect successed", addrs[0])
			} else {
				fmt.Println("connect failed", addrs[0])
			}
		}
		return nil, nil
	},
	/*
		"bootstrap": func(args ... string) (interface{}, error) {
			reqest, err := http.NewRequest("GET", funcs_bootstrap_url(), nil)
			if err != nil {
				log4go.Error(err)
				return nil, err
			}
			response, _ := client.Do(reqest)
			defer response.Body.Close()
			success := rpcserver.SuccessFromReader(response.Body)
			if success != nil && success.Success {
				fmt.Println("bootstrap successed")
			} else {
				fmt.Println("bootstrap failed")
			}
			return nil, err
		},
	*/
	"put": func(args ...string) (interface{}, error) {
		if len(args) != 2 {
			return nil, errors.New("fail params")
		}
		key, value := args[0], args[1]
		reqest, err := http.NewRequest("GET", funcs_put_url(key, value), nil)
		if err != nil {
			log4go.Error(err)
			return nil, err
		}
		response, _ := client.Do(reqest)
		if response != nil && response.Body != nil {
			defer response.Body.Close()
		}
		success := rpcserver.SuccessFromReader(response.Body)
		if success != nil && success.Success {
			fmt.Println("put successed")
		} else {
			fmt.Println("put failed")
		}
		return nil, err
	},
	"get": func(args ...string) (interface{}, error) {
		if len(args) != 1 {
			return nil, errors.New("fail params")
		}
		key := args[0]
		reqest, err := http.NewRequest("GET", funcs_get_url(key), nil)
		if err != nil {
			log4go.Error(err)
			return nil, err
		}
		response, _ := client.Do(reqest)
		if response != nil && response.Body != nil {
			defer response.Body.Close()
		}
		success := rpcserver.SuccessFromReader(response.Body)
		if success != nil && success.Success {
			fmt.Println("get successed")
			return success.Entity, nil
		} else {
			fmt.Println("get failed")
		}
		return nil, nil
	},
	"bing": func(args ...string) (interface{}, error) {
		if len(args) != 2 {
			return nil, errors.New("fail params")
		}
		bo, to := args[0], args[1]
		reqest, err := http.NewRequest("GET", funcs_bing_url(bo, to), nil)
		if err != nil {
			log4go.Error(err)
			return nil, err
		}
		response, _ := client.Do(reqest)
		if response != nil && response.Body != nil {
			defer response.Body.Close()
		}
		success := rpcserver.SuccessFromReader(response.Body)
		if success != nil && success.Success {
			fmt.Println("bing successed")
			return success.Entity, nil
		} else {
			fmt.Println("bing failed")
		}
		return nil, nil
	},
	"ping": func(args ...string) (interface{}, error) {
		if len(args) != 1 {
			return nil, errors.New("fail params")
		}
		to := args[0]
		reqest, err := http.NewRequest("GET", funcs_ping_url(to), nil)
		if err != nil {
			log4go.Error(err)
			return nil, err
		}
		response, _ := client.Do(reqest)
		if response != nil && response.Body != nil {
			defer response.Body.Close()
		}
		success := rpcserver.SuccessFromReader(response.Body)
		fmt.Println("ping_success =", success)
		if success != nil && success.Success {
			fmt.Println("ping successed")
			return success.Entity, nil
		} else {
			fmt.Println("ping failed")
		}
		return nil, nil
	},
	"getastab": func(args ...string) (interface{}, error) {
		success := &rpcserver.Success{}
		id := ""
		if len(args) == 1 {
			id = args[0]
		}
		reqest, err := http.NewRequest("GET", funcs_getastab_url(id), nil)
		if err != nil {
			log4go.Error(err)
			return nil, err
		}
		response, _ := client.Do(reqest)
		if response != nil && response.Body != nil {
			defer response.Body.Close()
		}
		success = rpcserver.SuccessFromReader(response.Body)

		if success != nil && success.Success {
			am := success.Entity.(map[string]interface{})
			for protoID, peerArray := range am {
				fmt.Println(protoID)
				for i, p := range peerArray.([]interface{}) {
					fmt.Println("\t", i, p)
				}
			}
		} else {
			fmt.Println("getastab failed")
		}
		return nil, nil
	},
	"local": func(args ...string) (interface{}, error) {
		if len(args) != 1 {
			return nil, errors.New("fail params")
		}
		to := args[0]
		reqest, err := http.NewRequest("GET", funcs_local_url(to), nil)
		if err != nil {
			log4go.Error(err)
			return nil, err
		}
		response, _ := client.Do(reqest)
		if response != nil && response.Body != nil {
			defer response.Body.Close()
		}
		success := rpcserver.SuccessFromReader(response.Body)
		if success != nil && success.Success {
			fmt.Println("local successed")
			return success.Entity, nil
		} else {
			fmt.Println("local failed")
		}
		return nil, nil
	},
	"findpeer": func(args ...string) (interface{}, error) {
		if len(args) != 1 {
			return nil, errors.New("fail params")
		}
		to := args[0]
		reqest, err := http.NewRequest("GET", funcs_findpeer_url(to), nil)
		if err != nil {
			log4go.Error(err)
			return nil, err
		}
		response, _ := client.Do(reqest)
		if response != nil && response.Body != nil {
			defer response.Body.Close()
		}
		success := rpcserver.SuccessFromReader(response.Body)
		fmt.Println("->", success)
		return nil, err
	},

	"getfile": func(args ...string) (interface{}, error) {
		if len(args) != 3 {
			return nil, errors.New("fail params")
		}
		targetid, filepath, output := args[0], args[1], args[2]
		reqest, err := http.NewRequest("GET", funcs_getfile_url(filepath, targetid, output), nil)
		if err != nil {
			log4go.Error(err)
			return nil, err
		}
		response, _ := client.Do(reqest)
		if response != nil && response.Body != nil {
			defer response.Body.Close()
		}
		success := rpcserver.SuccessFromReader(response.Body)
		fmt.Println("->", success)
		return nil, err
	},

	/*
		"scp": func(args ... string) (interface{}, error) {
			if len(args) != 2 {
				return nil, errors.New("fail params")
			}
			to, fp := args[0], args[1]
			tid, err := peer.IDB58Decode(to)
			if err != nil {
				return nil, err
			}
			s, err := node.Host().NewStream(context.Background(), tid, P_CHANNEL_FILE)
			if err != nil {
				return nil, err
			}
			buff, err := ioutil.ReadFile(fp)
			if err != nil {
				return nil, err
			}
			l := int64(len(buff))
			lenBuff := bytes.NewBuffer([]byte{})
			binary.Write(lenBuff, binary.BigEndian, l)
			head := lenBuff.Bytes()
			fmt.Println("head --> ", len(head), head)
			s.Write(head)
			i, err := s.Write(buff)
			log4go.Info("wait feedback. %d", i)
			res := make([]byte, 8)
			if i, e := s.Read(res); e == nil {
				total := new(big.Int).SetBytes(res[0:i])
				log4go.Info("(%d) write byte : %d , remote recv : %d", i, l, total.Int64())
			} else {
				log4go.Error(e)
			}
			s.Close()
			return nil, err
		},
		"relay": func(args ... string) (interface{}, error) {
			if len(args) != 2 {
				return nil, errors.New("fail params")
			}
			from, to := args[0], args[1]
			fid, err := peer.IDB58Decode(from)
			if err != nil {
				return nil, err
			}
			tid, err := peer.IDB58Decode(to)
			if err != nil {
				return nil, err
			}
			addr, err := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s/p2p-circuit/ipfs/%s", fid.Pretty(), tid.Pretty()))
			if err != nil {
				log4go.Error(err)
			}
			node.Host().Peerstore().AddAddrs(tid, []ma.Multiaddr{addr}, pstore.TempAddrTTL)
			return nil, nil
		},
	*/
}
