package mobile

import (
	"context"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/agents"
	. "github.com/SmartMeshFoundation/Perception/cmd/utils"
	"github.com/SmartMeshFoundation/Perception/core"
	"github.com/SmartMeshFoundation/Perception/live"
	"github.com/SmartMeshFoundation/Perception/params"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"time"
)

var (
	agentClientCfg = "ipfsapi:45001,ipfsgateway:48080,rest:48095"
	coreNum        = 1
	ctx            = context.Background()
	client         = &http.Client{}
	getastab       = `http://localhost:40080/api/?body={"sn":"sn-101","service":"funcs","method":"getastab"}`
	exit           = `http://localhost:40080/api/?body={"sn":"sn-101","service":"funcs","method":"exit"}`
)

func Exit() {
	Node.Close()
}

func Getastab() string {
	reqest, err := http.NewRequest("GET", getastab, nil)
	if err != nil {
		log4go.Error(err)
		return ""
	}
	response, _ := client.Do(reqest)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if buf, err := ioutil.ReadAll(response.Body); err == nil {
		return string(buf)
	}
	return ""
}

func Start(homeDir, dataDir string, port int) {
	Start2(homeDir, dataDir, port, "error")
}

func Start2(homeDir, dataDir string, port int, loglevel string) {
	//reset stop chan
	Stop = make(chan struct{})
	switch loglevel {
	case "error":
		log4go.AddFilter("stdout", log4go.ERROR, log4go.NewConsoleLogWriter())
	case "debug":
		log4go.AddFilter("stdout", log4go.DEBUG, log4go.NewConsoleLogWriter())
	default:
		log4go.AddFilter("stdout", log4go.INFO, log4go.NewConsoleLogWriter())
	}

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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	Node = core.NewNode(ctx, cancel, prv, port)
	// TODO 实验阶段
	Node.SetLiveServer(live.NewLiveServer(Node, params.LivePort))

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
	fmt.Printf("myid : %s\n", Node.Host().ID().Pretty())
	fmt.Printf("myaddrs : %v\n", Node.Host().Network().ListenAddresses())
	fmt.Printf("homedir: %s\n", params.HomeDir)
	fmt.Printf("datadir: %s\n", params.DataDir)
	fmt.Printf("version: %s\n", params.VERSION)
	//log4go.Info("agent-client: %s", agentClientCfg)
	Node.Start(false)
	go AsyncActionLoop(ctx, Node)
	go func() {
		t := time.NewTicker(30 * time.Second)
		for range t.C {
			select {
			case <-t.C:
				log4go.Info("total_peer : %d", len(Node.Host().Network().Conns()))
			case <-ctx.Done():
				return
			}
		}
	}()
	<-Stop
}
