package agents

import (
	"context"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/agents/pb"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	inet "gx/ipfs/QmPtFaR7BWHLAjSwLh9kXcyrgTzDpuhcWLkx8ioa9RMYnx/go-libp2p-net"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"net"
	"sync"
	"time"
)

type AgentServerImpl struct {
	node  types.Node
	table *Astable
}

func NewAgentServer(astab *Astable) *AgentServerImpl {
	log4go.Info("web3 rpc agent : %s", Web3RpcAgentConfig)
	log4go.Info("web3 ws agent : %s", Web3WsAgentConfig)
	log4go.Info("ipfs api agent : %s", IpfsApiAgentConfig)
	log4go.Info("ipfs gateway agent : %s", IpfsGatewayAgentConfig)
	log4go.Info("rest agent: %s", RestAgentConfig)
	return &AgentServerImpl{astab.node, astab}
}

func (self *AgentServerImpl) Start() {
	ast := self.table
	myid := self.node.Host().ID()
	// flush agent-server table
	am := agents_pb.NewMessage(agents_pb.AgentMessage_ADD_AS_TAB)
	if Web3RpcAgentConfig != "" {
		ast.Append(params.P_AGENT_WEB3_RPC, myid)
		am.Append(params.P_AGENT_WEB3_RPC, myid)
	}
	if Web3WsAgentConfig != "" {
		ast.Append(params.P_AGENT_WEB3_WS, myid)
		am.Append(params.P_AGENT_WEB3_WS, myid)
	}
	if IpfsApiAgentConfig != "" {
		ast.Append(params.P_AGENT_IPFS_API, myid)
		am.Append(params.P_AGENT_IPFS_API, myid)
	}
	if IpfsGatewayAgentConfig != "" {
		ast.Append(params.P_AGENT_IPFS_GATEWAY, myid)
		am.Append(params.P_AGENT_IPFS_GATEWAY, myid)
	}
	if RestAgentConfig != "" {
		ast.Append(params.P_AGENT_REST, myid)
		am.Append(params.P_AGENT_REST, myid)
	}
	if am.AgentServerList != nil {
		log4go.Info("Broadcast self as agent-server in new thread.")
		go ast.KeepBroadcast(context.Background(), myid, am, params.AgentServerBroadcastInterval)
	}
}

func (self *AgentServerImpl) IpfsAgent(c inet.Stream, cfg types.AgentCfg) {
	self.handAgent(c, cfg)
}

func (self *AgentServerImpl) Web3Agent(c inet.Stream, cfg types.AgentCfg) {
	self.handAgent(c, cfg)
}
func (self *AgentServerImpl) RestAgent(c inet.Stream, cfg types.AgentCfg) {
	// TODO 1 跳
	self.handAgent(c, cfg)
}

func (self *AgentServerImpl) handAgent(c inet.Stream, cfg types.AgentCfg) {
	_, target, timeout, _ := cfg.Open()
	setTimeOut := func(sc inet.Stream, tc net.Conn) error {
		if timeout <= 0 {
			return nil
		}
		if sc != nil {
			err := sc.SetDeadline(time.Now().Add(time.Second * time.Duration(timeout)))
			if err != nil {
				return err
			}
		}
		if tc != nil {
			err := tc.SetDeadline(time.Now().Add(time.Second * time.Duration(timeout)))
			if err != nil {
				return err
			}
		}
		return nil
	}
	handleConn := func(sc inet.Stream) {
		wg := new(sync.WaitGroup)
		defer func() {
			fmt.Println("ByeBye.")
			if sc != nil {
				sc.Close()
			}
		}()

		tc, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", target))
		if err != nil {
			log4go.Error(err)
			return
		}
		defer func() {
			if tc != nil {
				tc.Close()
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, BufferSize)
			for {
				i, err := sc.Read(buf)
				setTimeOut(sc, tc)
				if err != nil {
					log4go.Error(err)
					tc.Close()
					return
				}
				_, err = tc.Write(buf[:i])
				if err != nil {
					log4go.Error(err)
					return
				}
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, BufferSize)
			for {
				i, err := tc.Read(buf)
				setTimeOut(sc, tc)
				if err != nil {
					log4go.Error(err)
					return
				}
				_, err = sc.Write(buf[:i])
				if err != nil {
					log4go.Error(err)
					return
				}
			}
		}()

		log4go.Info("-> channel build successed.")
		wg.Wait()
	}

	err := setTimeOut(c, nil)
	if err != nil {
		log4go.Error(err)
	}
	handleConn(c)
}
