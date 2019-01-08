package agents

import (
	"context"
	"errors"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/agents/pb"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	inet "gx/ipfs/QmPtFaR7BWHLAjSwLh9kXcyrgTzDpuhcWLkx8ioa9RMYnx/go-libp2p-net"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	"math/big"
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

func (self *AgentServerImpl) SetupReport(cfg types.AgentCfg) error {
	n, _, _, err := cfg.Open()
	if err != nil {
		return err
	}

	switch protocol.ID(n) {
	case params.P_AGENT_IPFS_API, params.P_AGENT_IPFS_GATEWAY:
		enableReport("ipfs")
	default:
		return errors.New("not_support_report_yet : " + n)
	}
	return nil
}

func (self *AgentServerImpl) FetchReport(k, startDate, endDate string) (map[string]interface{}, error) {
	return Report(k, startDate, endDate), nil
}

func (self *AgentServerImpl) Start() {
	log4go.Info("AgentServer_ready_for_start.")
	ast := self.table
	// 要等 astable.Start 成功了之后
	ast.wg.Wait()
	log4go.Info("AgentServer_start_successed.")

	myid := self.node.Host().ID()
	// flush agent-server table
	am := agents_pb.NewMessage(agents_pb.AgentMessage_ADD_AS_TAB)
	go func() {
		location := types.NewGeoLocation(-206, -206)
		location.ID = myid
		// 如果没有 geo 这需要后续问其他 as 询问了, 询问完成之前没必要广播自己的信息
		if selfgeo := self.node.GetGeoLocation(); selfgeo != nil {
			am.Location = &agents_pb.AgentMessage_Location{
				Longitude: float32(selfgeo.Longitude),
				Latitude:  float32(selfgeo.Latitude),
				Peer:      []byte(myid),
			}
			log4go.Info("🛰️ Broadcast_AS_info_take_Location : %v", selfgeo)
			location = self.node.GetGeoLocation()
		} else {
			log4go.Info("🛰️ ❌ : %v", selfgeo)
		}

		if Web3RpcAgentConfig != "" {
			ast.Append(params.P_AGENT_WEB3_RPC, location)
			am.Append(params.P_AGENT_WEB3_RPC, location)
		}
		if Web3WsAgentConfig != "" {
			ast.Append(params.P_AGENT_WEB3_WS, location)
			am.Append(params.P_AGENT_WEB3_WS, location)
		}
		if IpfsApiAgentConfig != "" {
			ast.Append(params.P_AGENT_IPFS_API, location)
			am.Append(params.P_AGENT_IPFS_API, location)
		}
		if IpfsGatewayAgentConfig != "" {
			ast.Append(params.P_AGENT_IPFS_GATEWAY, location)
			am.Append(params.P_AGENT_IPFS_GATEWAY, location)
		}
		if RestAgentConfig != "" {
			ast.Append(params.P_AGENT_REST, location)
			am.Append(params.P_AGENT_REST, location)
		}
		if am.AgentServerList != nil {
			log4go.Info("Broadcast self as agent-server in new thread.")
			go ast.KeepBroadcast(context.Background(), myid, am, params.AgentServerBroadcastInterval)
		}
	}()
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
	name, target, timeout, _ := cfg.Open()
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
			total := 0
			defer func() {
				Record(name, big.NewInt(int64(total)))
				wg.Done()
			}()

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
				total += i
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
