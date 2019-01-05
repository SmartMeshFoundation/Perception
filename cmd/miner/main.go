package main

import (
	"github.com/cc14514/go-geoip2-db"
	"github.com/SmartMeshFoundation/Perception/agents"
	. "github.com/SmartMeshFoundation/Perception/cmd/utils"
	"github.com/SmartMeshFoundation/Perception/core"
	"github.com/SmartMeshFoundation/Perception/params"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"os"
	"runtime"
)

var (
	stop           = make(chan struct{})
	agentServerCfg = "ipfsapi:5001,ipfsgateway:8080"
)

func main() {
	log4go.AddFilter("stdout", log4go.INFO, log4go.NewConsoleLogWriter())
	coreNum := runtime.NumCPU()
	log4go.Info("cpu core used : %d", coreNum)
	runtime.GOMAXPROCS(coreNum)
	Start(params.HomeDir, "", params.DefaultPort)
}

func Start(homeDir, dataDir string, port int) {

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

	// as 启动 geoipdb ac 不启动 >>>>>>>>>>
	geoipdb, err := geoip2db.NewGeoipDbByStatik()
	if err == nil {
		Node.SetGeoipDB(geoipdb)
	}
	// as 启动 geoipdb ac 不启动 <<<<<<<<<<

	Astab = agents.NewAstable(Node)
	err = agents.SetAgentsConfig(agentServerCfg)
	if err != nil {
		log4go.Error(err)
		return
	}
	agentServer := agents.NewAgentServer(Astab)
	agentServer.SetupReport(agents.IpfsApiAgentConfig)
	Node.SetAgentServer(agentServer)

	log4go.Info("myid : %s", Node.Host().ID().Pretty())
	log4go.Info("myaddrs : %v", Node.Host().Network().ListenAddresses())
	log4go.Info("homedir: %s", params.HomeDir)
	log4go.Info("datadir: %s", params.DataDir)
	log4go.Info("agent-server: %s", agentServer)
	Node.Start(false)

	<-stop
}
