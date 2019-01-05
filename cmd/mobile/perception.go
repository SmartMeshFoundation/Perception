package mobile

import (
	"github.com/SmartMeshFoundation/Perception/agents"
	. "github.com/SmartMeshFoundation/Perception/cmd/utils"
	"github.com/SmartMeshFoundation/Perception/core"
	"github.com/SmartMeshFoundation/Perception/params"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"os"
	"runtime"
)

var (
	agentClientCfg = "ipfsapi:45001,ipfsgateway:48080,rest:48095"
	coreNum        = 1
)

func Start(homeDir, dataDir string, port int) {

	log4go.AddFilter("stdout", log4go.INFO, log4go.NewConsoleLogWriter())

	if runtime.NumCPU() > 2 {
		coreNum = runtime.NumCPU() / 2
	}

	log4go.Info("cpu core used : %d", coreNum)
	runtime.GOMAXPROCS(coreNum)

	if homeDir != params.HomeDir {
		params.SetHomeDir(homeDir)
	}
	if dataDir != "" {
		params.SetDataDir(dataDir)
	}

	prv, err := core.LoadKey(homeDir)
	if err != nil {
		os.Mkdir(homeDir, 0755)
		prv, _ = core.GenKey(homeDir)
	}

	Node = core.NewNode(prv, port)

	Astab = agents.NewAstable(Node)
	// StartAgentClient >>>>
	err = agents.SetAgentsConfig(agentClientCfg)
	if err != nil {
		log4go.Error(err)
		return
	}
	agentClient := agents.NewAgentClient(Astab)
	Node.SetAgentClient(agentClient)
	// StartAgentClient <<<<
	log4go.Info("myid : %s", Node.Host().ID().Pretty())
	log4go.Info("myaddrs : %v", Node.Host().Network().ListenAddresses())
	log4go.Info("homedir: %s", params.HomeDir)
	log4go.Info("datadir: %s", params.DataDir)
	log4go.Info("agent-client: %s", agentClientCfg)
	Node.Start(false)
	stop := make(chan struct{})
	<-stop
}