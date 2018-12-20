package agents

import (
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
)

type AgentCfgImpl string

var (
	// init by command line flags
	// format : "port,timeout"
	Web3WsAgentConfig      AgentCfgImpl = ""
	Web3RpcAgentConfig     AgentCfgImpl = ""
	IpfsApiAgentConfig     AgentCfgImpl = ""
	IpfsGatewayAgentConfig AgentCfgImpl = ""

	RestAgentConfig AgentCfgImpl = ""

	BufferSize = 256 * 1024
)

//web3rpc:18545,web3ws:18546,ipfsapi:8088,ipfsgateway:8010
func SetAgentsConfig(val string) error {
	if val == "" {
		return nil
	}
	as := strings.Split(val, ",")
	for _, a := range as {
		bs := strings.Split(a, ":")
		if len(bs) != 2 {
			return errors.New(fmt.Sprintf("fail AgentCfg : %s", val))
		}
		switch bs[0] {
		case "web3rpc":
			Web3RpcAgentConfig = AgentCfgImpl(fmt.Sprintf("%s,%s,%d", bs[0], bs[1], 15))
		case "web3ws":
			Web3WsAgentConfig = AgentCfgImpl(fmt.Sprintf("%s,%s,%d", bs[0], bs[1], 30))
		case "ipfsapi":
			IpfsApiAgentConfig = AgentCfgImpl(fmt.Sprintf("%s,%s,%d", bs[0], bs[1], -1))
		case "ipfsgateway":
			IpfsGatewayAgentConfig = AgentCfgImpl(fmt.Sprintf("%s,%s,%d", bs[0], bs[1], -1))
		case "rest":
			RestAgentConfig = AgentCfgImpl(fmt.Sprintf("%s,%s,%d", bs[0], bs[1], -1))
		default:
			return errors.New(fmt.Sprintf("fail AgentCfg : %s", val))
		}
	}
	return nil
}

func (self AgentCfgImpl) Open() (name string, port, timeout int, err error) {
	arr := strings.Split(string(self), ",")
	if len(arr) != 3 {
		err = errors.New(fmt.Sprintf("fail AgentCfg : %s", self))
		return
	}
	p, err := strconv.ParseInt(arr[1], 10, 64)
	if err != nil {
		return
	}
	t, err := strconv.ParseInt(arr[2], 10, 64)
	if err != nil {
		return
	}
	name = path.Join("/agent", arr[0])
	port = int(p)
	timeout = int(t)
	return
}
