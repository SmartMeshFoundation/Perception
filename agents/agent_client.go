package agents

import (
	"context"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	inet "gx/ipfs/QmPtFaR7BWHLAjSwLh9kXcyrgTzDpuhcWLkx8ioa9RMYnx/go-libp2p-net"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	"net"
	"reflect"
	"runtime"
	"sync"
	"time"
)

type AgentClientImpl struct {
	node  types.Node
	table *Astable
}

func NewAgentClient(astab *Astable) *AgentClientImpl {
	log4go.Info("web3 rpc agent : %s", Web3RpcAgentConfig)
	log4go.Info("web3 ws agent : %s", Web3WsAgentConfig)
	log4go.Info("ipfs api agent : %s", IpfsApiAgentConfig)
	log4go.Info("ipfs gateway agent : %s", IpfsGatewayAgentConfig)
	ac := &AgentClientImpl{astab.node, astab}
	return ac
}

func (self *AgentClientImpl) Start() {
	if Web3RpcAgentConfig != "" {
		go self.Web3Agent(Web3RpcAgentConfig)
	}
	if Web3WsAgentConfig != "" {
		go self.Web3Agent(Web3WsAgentConfig)
	}
	if IpfsApiAgentConfig != "" {
		go self.IpfsAgent(IpfsApiAgentConfig)
	}
	if IpfsGatewayAgentConfig != "" {
		go self.IpfsAgent(IpfsGatewayAgentConfig)
	}
	if RestAgentConfig != "" {
		go self.RestAgent(RestAgentConfig)
	}
}

func (self *AgentClientImpl) RestAgent(cfg types.AgentCfg) {
	self.runagent(cfg)
}

func (self *AgentClientImpl) IpfsAgent(cfg types.AgentCfg) {
	self.runagent(cfg)
}

func (self *AgentClientImpl) Web3Agent(cfg types.AgentCfg) {
	self.runagent(cfg)
}

func (self *AgentClientImpl) runagent(cfg types.AgentCfg) {
	ctx := context.Background()
	name, port, timeout, _ := cfg.Open()
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	log4go.Info("start web3agent-client on %d : %s -->", port, name)
	if err != nil {
		log4go.Error(err)
		return
	}

	for {
		c, err := l.Accept()
		if err != nil {
			log4go.Error(err)
			break
		}
		err = setTimeOut(timeout, c)
		if err != nil {
			log4go.Error(err)
		}
		ctx = context.WithValue(ctx, "timeout", timeout)
		go self.handleConn(ctx, name, c)
	}
}
func setTimeOut(timeout int, conns ...interface{}) error {
	if timeout <= 0 {
		return nil
	}
	for _, conn := range conns {
		v := reflect.ValueOf(conn)
		timeout := time.Now().Add(time.Second * time.Duration(timeout))
		v.MethodByName("SetDeadline").Call([]reflect.Value{reflect.ValueOf(timeout)})
	}
	return nil
}

func (self *AgentClientImpl) handleConn(ctx context.Context, proto string, conn net.Conn) {
	p := protocol.ID(proto)
	log4go.Info("--> accept : %s", proto)
	switch p {
	case params.P_AGENT_REST, params.P_AGENT_WEB3_RPC, params.P_AGENT_WEB3_WS, params.P_AGENT_IPFS_API, params.P_AGENT_IPFS_GATEWAY:
		log4go.Info("--> do_accept : %v", p)
		ctx = context.WithValue(ctx, "protocol", p)
		self.rpchandler(ctx, conn)
	default:
		log4go.Error("fail protocol of agent client : %v", p)
		return
	}
}

/*
from						bridge				  	to
|							|						|
|---1 AgentMessage_BRIDGE-->|						|
|							|-----2 NewStream------>|
|<--3 AgentMessage_BRIDGE---|						|
|							|						|
|<------------r/w---------->|<----------r/w-------->|
|							|						|
*/
/*
func (self *AgentClientImpl) GenBridge(ctx context.Context, bid, tid peer.ID, pid protocol.ID) (inet.Stream, error) {
	log4go.Info("-[ 👬 gen agent bridge 👬 ]-> (%s) --> (%s) ", bid.Pretty(), tid.Pretty())
	am := agents_pb.NewMessage(agents_pb.AgentMessage_BRIDGE)
	as := new(agents_pb.AgentMessage_AgentServer)
	as.Pid = []byte(pid)
	as.Peers = [][]byte{[]byte(tid)}
	am.AgentServer = as
	stream, err := self.node.Host().NewStream(ctx, bid, params.P_CHANNEL_AGENTS)
	if err != nil {
		return nil, err
	}
	// 1
	rm, err := self.table.SendMsgByStream(ctx, stream, am)
	if err != nil {
		return nil, err
	}
	// 3
	if rm.Type == agents_pb.AgentMessage_BRIDGE {
		log4go.Info("agent_bridge_handshack_success %s : %s -> %s", pid, bid.Pretty(), tid.Pretty())
		return stream, nil
	}
	return nil, errors.New("error type")
}
*/

func (self *AgentClientImpl) rpchandler(ctx context.Context, sc net.Conn) {
	findby := make(chan peer.ID)
	var tc inet.Stream
	wg := new(sync.WaitGroup)
	defer func() {
		if sc != nil {
			sc.Close()
		}
		runtime.GC()
	}()
	proto := ctx.Value("protocol")
	if proto == nil {
		return
	}
	p := proto.(protocol.ID)
	timeout := ctx.Value("timeout")
	if timeout == nil {
		return
	}
	tout := timeout.(int)
	log4go.Info("-> timeout=%d", tout)

	// TODO 70 client 固定通道测试
	target, err := self.table.Fetch(p)
	log4go.Info("-> target=%v , err=%v", target.Pretty(), err)
	// target, err = peer.IDB58Decode("QmNNC87W9kWhn4UZBKMGr9APpk4KDN1ERC5YJqvwX1QxwK")
	//target, err := peer.IDB58Decode("QmXu6Cu5CpqBfCV9Pw9jQN8obwV3nUCVuvfaXhtRpomZGp")
	//log4go.Info("set_test_target --> %s", target.Pretty())
	if err != nil {
		sc.Write([]byte(err.Error()))
		return
	}

	pi, err := self.node.FindPeer(ctx, target, findby)
	if err != nil {
		sc.Write([]byte(err.Error()))
		return
	}
	// 尝试直连
	if err = self.node.Host().Connect(ctx, pi); err != nil {
		// 尝试搭桥 这里一定有返回值
		fy := <-findby
		if fy == "" {
			sc.Write([]byte(err.Error()))
			return
		}
		bridgeId := fy.Pretty()
		log4go.Info(" 👷‍ try_agent_brige_service : %s --> %s ", bridgeId, target)
		tc, err = self.table.GenBridge(ctx, fy, target, p)
	} else {
		log4go.Info(" 🌞 normal_%s_stream : --> %s", p, target)
		tc, err = self.node.Host().NewStream(ctx, target, p)
	}
	//tc, err := self.node.Host().NewStream(ctx, target, p)
	log4go.Info("-> target=%v, p=%v, err=%v", target.Pretty(), p, err)
	if err != nil {
		sc.Write([]byte(err.Error()))
		sc.Close()
		return
	}
	defer func() {
		if tc != nil {
			tc.Close()
		}
	}()
	log4go.Info("%s - channel -> %s succesed , timeout %d", p, target.Pretty(), tout)
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := make([]byte, BufferSize)
		for {
			i, err := sc.Read(buf)
			setTimeOut(tout, sc, tc)
			if err != nil {
				log4go.Error(err)
				tc.Close()
				return
			}
			log4go.Debug("--> in\n\n%s\n\n", buf[:i])
			_, err = tc.Write(buf[:i])
			if err != nil {
				sc.Close()
				log4go.Error(err)
				return
			}

		}
	}()

	go func() {
		defer wg.Done()
		buf := make([]byte, BufferSize)
		t, u := 0, 0
		for {
			i, err := tc.Read(buf)
			setTimeOut(tout, sc, tc)
			if err != nil {
				log4go.Error(err)
				sc.Close()
				return
			}
			log4go.Debug("<-- out\n\n%s\n\n", buf[:i])
			_, err = sc.Write(buf[:i])
			if err != nil {
				log4go.Error(err)
				tc.Close()
				return
			}
			t += i
			u += 1
			if u%10 == 0 {
				fmt.Println(time.Now().Unix(), "->", t)
			}
		}
	}()

	log4go.Info("%s -> agent_channel open .", proto)
	wg.Wait()
	log4go.Info("%s -> agent_channel close .", proto)
}
