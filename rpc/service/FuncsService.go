package service

import (
	"github.com/SmartMeshFoundation/Perception/tookit"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"time"

	"bytes"
	"context"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/agents/pb"
	"github.com/SmartMeshFoundation/Perception/cmd/utils"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	ma "gx/ipfs/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	"gx/ipfs/QmSzEdVLaPMQGAKKGo4mKjsbWcfz6w8CoDjhRPxdk7xYdn/go-ipfs-addr"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"gx/ipfs/QmYaVXmXZNpWs6owQ1rk5VAiwNinkTh2cYZuYx1JDSactL/go-lightrpc/rpcserver"
	"io"
	"os"
)

func init() {
}

type Funcs struct {
	node types.Node
}

func NewFuncs(node types.Node) *Funcs {
	return &Funcs{node}
}

func (self *Funcs) AsReport(requestParameter interface{}) rpcserver.Success {
	success := rpcserver.Success{Success: true}
	if self.node.GetAgentServer() == nil {
		success.Success = false
		success.Error("0", "not support")
		return success
	}
	var key, start, end string

	args := requestParameter.(map[string]interface{})
	log4go.Info("input--> %v", args)
	if v, ok := args["key"]; ok {
		key = v.(string)
	} else {
		key = "ipfs" // default
	}
	if v, ok := args["start"]; ok {
		start = v.(string)
	} else {
		start = time.Now().Format("20060102")
	}
	if v, ok := args["end"]; ok {
		end = v.(string)
	} else {
		end = time.Now().Format("20060102")
	}
	log4go.Info("key=%s , start=%s , end=%s", key, start, end)
	r, err := self.node.GetAgentServer().FetchReport(key, start, end)
	if err != nil {
		success.Success = false
		success.Error("1", err.Error())
		return success
	}
	success.Entity = r
	log4go.Info("output<-- %v", success)
	return success

}

func (self *Funcs) Myid(requestParameter interface{}) rpcserver.Success {
	if gl := self.node.GetGeoLocation(); gl != nil {
		return rpcserver.Success{
			Success: true,
			Entity: struct {
				Id                  string
				Vsn                 string
				Longitude, Latitude float64
			}{gl.ID.Pretty(), params.VERSION, gl.Longitude, gl.Latitude},
		}
	}
	return rpcserver.Success{
		Success: true,
		Entity: struct {
			Id  string
			Vsn string
		}{self.node.Host().ID().Pretty(), params.VERSION},
	}
}

func (self *Funcs) Myaddrs(requestParameter interface{}) rpcserver.Success {
	addrs := make([]string, 0)
	for _, addr := range self.node.Host().Addrs() {
		addrs = append(addrs, addr.String())
	}
	protocols, _ := self.node.Host().Peerstore().GetProtocols(self.node.Host().ID())
	return rpcserver.Success{
		Success: true,
		Entity:  struct{ Addrs, Protocols []string }{addrs, protocols},
	}
}

func (self *Funcs) Conns(requestParameter interface{}) rpcserver.Success {
	peers := make([]string, 0)
	for _, c := range self.node.Host().Network().Conns() {
		peers = append(peers, fmt.Sprintf("%s/ipfs/%s", c.RemoteMultiaddr().String(), c.RemotePeer().Pretty()))
	}
	return rpcserver.Success{
		Success: true,
		Entity: struct {
			Count int
			Peers []string
		}{len(peers), peers},
	}
}

func (self *Funcs) Peers(requestParameter interface{}) rpcserver.Success {
	peers := make([]string, 0)
	fmt.Println("->", len(self.node.Host().Network().Peerstore().Peers()))

	for _, p := range self.node.Host().Network().Peerstore().Peers() {
		if p == self.node.Host().ID() {
			continue
		}
		addrs := self.node.Host().Peerstore().Addrs(p)
		buf := bytes.NewBufferString(p.Pretty())
		if len(addrs) > 0 {
			buf.WriteString(fmt.Sprintf(" (addrs:%d)", len(addrs)))
			/*
				for _, a := range addrs {
					buf.WriteString(a.String())
					buf.WriteString(" , ")
				}
			*/
		}
		peers = append(peers, buf.String())
	}
	/*
		for _, c := range self.node.Host().Network().Conns() {
			peers = append(peers, fmt.Sprintf("%s/ipfs/%s\n", c.RemoteMultiaddr().String(), c.RemotePeer().Pretty()))
		}
	*/
	return rpcserver.Success{
		Success: true,
		Entity: struct {
			Count int
			Peers []string
		}{len(peers), peers},
	}
}

func (self *Funcs) Local(requestParameter interface{}) rpcserver.Success {
	success := &rpcserver.Success{Success: true}
	args := requestParameter.(map[string]interface{})
	id := args["id"].(string)
	pid, err := peer.IDB58Decode(id)
	if err != nil {
		success.Error("1", err)
		log4go.Error("addr_error : %s , %v", id, err)
	} else {
		addrs := self.node.Host().Peerstore().Addrs(pid)
		res := make([]string, len(addrs))
		for i, a := range addrs {
			/*
				for _, p := range a.Protocols() {
					fmt.Println(pid.Pretty(), "->", p.Name, p.Code)
				}
			*/
			res[i] = a.String()
		}
		success.Entity = res
	}
	return *success
}

func (self *Funcs) Conn(requestParameter interface{}) rpcserver.Success {
	success := &rpcserver.Success{Success: true}
	args := requestParameter.(map[string]interface{})
	a := args["addr"].(string)
	addr, err := ipfsaddr.ParseString(a)
	if err != nil {
		success.Error("1", err)
		fmt.Println("addr_error :", a, err)
	} else {
		err = self.node.Connect(context.Background(), string(addr.ID()), []ma.Multiaddr{addr.Transport()})
		if err != nil {
			success.Error("2", err)
			fmt.Println("conn_error :", a, err)
		} else {
			fmt.Println("success")
		}
	}
	return *success
}

func (self *Funcs) Bootstrap(requestParameter interface{}) rpcserver.Success {
	success := &rpcserver.Success{Success: true}
	err := self.node.Bootstrap(context.Background())
	if err != nil {
		success.Error("1", err)
		fmt.Println("bootstrap_error:", err)
	}
	return *success
}

func (self *Funcs) Put(requestParameter interface{}) rpcserver.Success {
	args := requestParameter.(map[string]interface{})
	key, value := args["key"].(string), args["value"].(string)
	success := &rpcserver.Success{Success: true}
	if err := self.node.PutValue(context.Background(), fmt.Sprintf("/cc14514/%s", key), []byte(value)); err != nil {
		success.Error("2", err)
		fmt.Println("put_error :", err)
	}
	return *success
}

func (self *Funcs) Get(requestParameter interface{}) rpcserver.Success {
	args := requestParameter.(map[string]interface{})
	key := args["key"].(string)
	success := &rpcserver.Success{Success: true}

	buf, err := self.node.GetValue(context.Background(), fmt.Sprintf("/cc14514/%s", key))
	if err != nil {
		success.Error("2", err)
		fmt.Println("put_error :", err)
	} else {
		success.Entity = string(buf)
	}
	return *success
}

func (self *Funcs) Getastab(requestParameter interface{}) rpcserver.Success {
	args := requestParameter.(map[string]interface{})
	iid, ok := args["id"]
	success := &rpcserver.Success{Success: true}
	// .entity = {protoID:[peer1,peer2...],...}
	entity := make(map[string][]string)
	if iid == "" || !ok {
		table := utils.Astab.GetTable()
		for protoID, ll := range table {
			arr := make([]string, 0)
			fmt.Println(protoID)
			selfgeo := self.node.GetGeoLocation()
			for e := ll.Front(); e != nil; e = e.Next() {
				gl, ok := e.Value.(*types.GeoLocation)
				if !ok {
					ll.Remove(e)
					continue
				}
				//p := e.Value.(peer.ID)
				item := fmt.Sprintf("%s (%f,%f)", gl.ID.Pretty(), gl.Latitude, gl.Longitude)
				if selfgeo != nil && tookit.VerifyLocation(gl.Latitude, gl.Longitude) {
					km := tookit.Distance(selfgeo.Latitude, selfgeo.Longitude, gl.Latitude, gl.Longitude)
					item = fmt.Sprintf("%s %.2fkm", item, km)
				}
				arr = append(arr[:], item)
			}
			entity[string(protoID)] = arr
		}
		success.Entity = entity
	} else {
		id := iid.(string)
		peerID, err := peer.IDB58Decode(id)
		msg := agents_pb.NewMessage(agents_pb.AgentMessage_GET_AS_TAB)
		self.node.Host().NewStream(context.Background(), peerID, params.P_CHANNEL_AGENTS)
		fmt.Println("-->", msg)
		respmsg, err := utils.Astab.SendMsg(context.Background(), peerID, msg)
		fmt.Println("<--", err)
		if err != nil {
			success.Success = false
			success.Error("2", err)
		} else {
			for _, as := range respmsg.AgentServerList {
				sp := string(as.Pid)
				arr := make([]string, 0)
				for _, location := range as.Locations {
					peerIDD := peer.ID(location.Peer)
					arr = append(arr[:], peerIDD.Pretty())
				}
				/*
					for _, bpeer := range as.Peers {
						peerIDD := peer.ID(bpeer)
						arr = append(arr[:], peerIDD.Pretty())
					}
				*/
				entity[sp] = arr
			}
			success.Entity = entity
		}
	}
	return *success
}

func (self *Funcs) Bing(requestParameter interface{}) rpcserver.Success {
	args := requestParameter.(map[string]interface{})
	log4go.Info("bing_requestParameter -> %v", args)
	bo := args["bo"].(string)
	to := args["to"].(string)
	success := &rpcserver.Success{Success: true}

	s, err := self.node.GetStreamGenerater().GenBridge(bo, to, params.BRIDGE_ACTION_PING)
	if err != nil {
		success.Error("1-5", err)
		return *success
	}
	i, err := s.Write([]byte("ping"))
	if err != nil {
		success.Error("6", err)
		fmt.Println("ping_error :", err)
	}
	log4go.Info("wait feedback. %d", i)
	res := make([]byte, 4)
	if i, e := s.Read(res); e == nil {
		log4go.Info("(%d) remote recv : %s", i, string(res))
		success.Entity = string(res)
		return *success
	} else {
		success.Error("7", err)
		fmt.Println("ping_error :", err)
	}
	return *success
}

func (self *Funcs) Ping(requestParameter interface{}) rpcserver.Success {
	args := requestParameter.(map[string]interface{})
	to := args["to"].(string)
	success := &rpcserver.Success{Success: true}
	s, err := self.node.GetStreamGenerater().GenPingStream(to)
	if err != nil {
		success.Error("1-5", err)
		return *success
	}
	i, err := s.Write([]byte("ping"))
	if err != nil {
		success.Error("6", err)
		log4go.Error("ping_error : %v", err)
	}
	log4go.Info("wait feedback. %d", i)
	res := make([]byte, 4)
	if i, e := s.Read(res); e == nil {
		log4go.Info("(%d) remote recv : %s", i, string(res))
		success.Entity = string(res)
		return *success
	} else {
		success.Error("7", e)
		log4go.Error("ping_error : %v", err)
	}
	return *success
}
func (self *Funcs) Findpeer(requestParameter interface{}) rpcserver.Success {
	ctx := context.Background()
	args := requestParameter.(map[string]interface{})
	to := args["to"].(string)
	success := &rpcserver.Success{Success: true}
	findby := make(chan peer.ID)
	pi, err := self.node.FindPeer(ctx, to, findby)
	if err != nil {
		success.Error("3", err)
		fmt.Println("findpeer_error :", err)
		return *success
	}
	fy := <-findby
	p1, _ := self.node.Host().Peerstore().GetProtocols(pi.ID)
	p2, _ := self.node.Host().Peerstore().GetProtocols(fy)
	fmt.Println("p1", p1)
	fmt.Println("p2", p2)
	r := []string{pi.ID.Pretty()}
	for _, a := range pi.Addrs {
		r = append(r, a.String())
	}

	success.Entity = map[string]interface{}{
		"pi":     r,
		"findby": fy.Pretty(),
	}
	return *success
}

func (self *Funcs) Getfile(requestParameter interface{}) rpcserver.Success {
	args := requestParameter.(map[string]interface{})
	filepath := args["filepath"].(string)
	targetid := args["targetid"].(string)
	output := args["output"].(string)
	success := &rpcserver.Success{Success: true}

	fse, err := self.node.GetStreamGenerater().GenFileStream(filepath, targetid, 0, 0)
	if err != nil {
		success.Error("0", err)
		return *success
	}
	defer func() {
		if fse != nil && fse.S != nil {
			log4go.Info("hash = %s", fse.Hash)
			fse.S.Close()
		}
	}()

	f, err := os.OpenFile(output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		success.Error("0", err)
		return *success
	}
	defer f.Close()

	buff := make([]byte, 4096)
	total := int64(0)
	for total < fse.Size {
		t, err := fse.S.Read(buff)
		if err != nil && err != io.EOF {
			log4go.Error(err)
			success.Error("0", err)
			return *success
		}
		if t <= 0 {
			log4go.Info("<- EOF ->")
			break
		}
		f.Write(buff[0:t])
		total += int64(t)
	}
	log4go.Info("-> success : total_size=%d , recv_size=%d , output=%d", fse.Size, total, output)
	return *success
}
