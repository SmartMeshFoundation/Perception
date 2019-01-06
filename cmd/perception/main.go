package main

import (
	"fmt"
	"github.com/SmartMeshFoundation/Perception/core"
	"github.com/SmartMeshFoundation/Perception/live"
	"github.com/SmartMeshFoundation/Perception/params"
	"github.com/cc14514/go-geoip2-db"
	"github.com/urfave/cli"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	logger "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"
	"gx/ipfs/QmV281Yximj5ftHwYMSRCLbFhErRxqUFP2F8Rfa19LeToz/go-libp2p/p2p/discovery"
	"os"
	"runtime"
	"time"
	//"gx/ipfs/QmQvJiADDe7JR4m968MwXobTCCzUqQkP87aRHe29MEBGHV/go-logging"
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/SmartMeshFoundation/Perception/agents"
	. "github.com/SmartMeshFoundation/Perception/cmd/utils"
	"net/http"
	_ "net/http/pprof"
	"strings"
)

var log = logger.Logger("main")

var coreNum = 2

func init() {
	if runtime.NumCPU() > coreNum*8 {
		coreNum = runtime.NumCPU() / 8
	}
	log4go.Info("cpu core used : %d", coreNum)
	runtime.GOMAXPROCS(coreNum)

	Stop = make(chan struct{})
	App = cli.NewApp()
	App.Name = os.Args[0]
	App.Usage = "P2P network program"
	App.Version = params.VERSION
	App.Author = params.AUTHOR
	App.Email = params.EMAIL
	App.Flags = Flags

	App.Action = func(ctx *cli.Context) error {
		start(ctx)
		go func() {
			t := time.NewTicker(10 * time.Second)
			for range t.C {
				log4go.Info("total peer : %d", len(Node.Host().Network().Conns()))
				for i, c := range Node.Host().Network().Conns() {
					log4go.Debug("%d -> %s/ipfs/%s", i, c.RemoteMultiaddr().String(), c.RemotePeer().Pretty())
				}
			}
		}()
		<-Stop
		return nil
	}
	App.Before = func(ctx *cli.Context) error {
		if HOME_DIR != params.HomeDir {
			params.SetHomeDir(HOME_DIR)
		}
		if DATA_DIR != "" {
			params.SetDataDir(DATA_DIR)
		}
		if NETWORK_ID != "" {
			params.SetNetworkID(NETWORK_ID)
		}
		params.HTTPPort = ctx.GlobalInt("rpcport")

		idx := ctx.GlobalInt("loglevel")
		level := LogLevel[idx]
		log4go.AddFilter("stdout", log4go.Level(level), log4go.NewConsoleLogWriter())
		return nil
	}
	App.After = func(ctx *cli.Context) error {
		log4go.Close()
		return nil
	}
	App.Commands = []cli.Command{
		{
			Name: "version",
			Action: func(ctx *cli.Context) error {
				fmt.Println("version\t:", App.Version)
				fmt.Println("auth\t:", App.Author)
				fmt.Println("email\t:", App.Email)
				fmt.Println("source\t:", params.SOURCE)
				return nil
			},
		},
		{
			Name: "genkey",
			Action: func(ctx *cli.Context) error {
				log4go.Close()
				key := make([]byte, 32)
				_, err := rand.Read(key)
				if err != nil {
					fmt.Println("While trying to read random source:", err)
				}
				fmt.Println(hex.EncodeToString(key))
				return nil
			},
		},
		{
			Name:   "console",
			Usage:  "一个简单的交互控制台，用来调试",
			Action: ConsoleCmd,
		},
		{
			Name:   "attach",
			Usage:  "连接本地已启动的进程",
			Action: AttachCmd,
		},
	}
}

func main() {
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()
	if err := App.Run(os.Args); err != nil {
		fmt.Println(err)
	}
}

func start(ctx *cli.Context) {
	prv, err := core.LoadKey(params.HomeDir)
	if err != nil {
		os.Mkdir(params.HomeDir, 0755)
		prv, _ = core.GenKey(params.HomeDir)
	}
	port := ctx.GlobalInt("port")

	Node = core.NewNode(prv, port)

	Astab = agents.NewAstable(Node)
	if AGENTSERVER != "" {

		// as 启动 geoipdb ac 不启动 >>>>>>>>>>
		geoipdb, err := geoip2db.NewGeoipDbByStatik()
		if err == nil {
			Node.SetGeoipDB(geoipdb)
		}
		// as 启动 geoipdb ac 不启动 <<<<<<<<<<

		err = agents.SetAgentsConfig(AGENTSERVER)
		if err != nil {
			log4go.Error(err)
			return
		}
		agentServer := agents.NewAgentServer(Astab)
		Node.SetAgentServer(agentServer)
	} else if AGENTCLIENT != "" {
		err := agents.SetAgentsConfig(AGENTCLIENT)
		if err != nil {
			log4go.Error(err)
			return
		}
		agentClient := agents.NewAgentClient(Astab)
		Node.SetAgentClient(agentClient)
	}

	c := context.Background()
	// setup mdns
	ds, err := discovery.NewMdnsService(c, Node.Host(), time.Second*5, discovery.ServiceTag)
	if err != nil {
		log4go.Error("mdns error : %v", err)
	} else {
		Node.SetDiscovery(ds)
	}

	// TODO 实验阶段
	Node.SetLiveServer(live.NewLiveServer(Node, params.DefaultLivePort))

	log4go.Info("myid : %s", Node.Host().ID().Pretty())
	log4go.Info("myaddrs : %v", Node.Host().Network().ListenAddresses())
	log4go.Info("homedir: %s", params.HomeDir)
	log4go.Info("datadir: %s", params.DataDir)
	Node.Start(false)
	go AsyncActionLoop(c, Node)
}

func AttachCmd(ctx *cli.Context) error {
	<-time.After(time.Second)
	go func() {
		defer close(Stop)
		fmt.Println("------------")
		fmt.Println("hello world")
		fmt.Println("------------")
		for {
			fmt.Print("cmd #>")
			ir := bufio.NewReader(os.Stdin)
			if cmd, err := ir.ReadString('\n'); err == nil && strings.Trim(cmd, " ") != "\n" {
				cmd = strings.Trim(cmd, " ")
				cmd = cmd[:len([]byte(cmd))-1]
				// TODO 用正则表达式拆分指令和参数
				cmdArg := strings.Split(cmd, " ")
				switch cmdArg[0] {
				case "exit", "quit":
					fmt.Println("bye bye ^_^ ")
					return
				case "help", "bootstrap":
					if _, err := Funcs[cmdArg[0]](); err != nil {
						log4go.Error(err)
					}
				default:
					log4go.Debug(cmdArg[0])
					if fn, ok := Funcs[cmdArg[0]]; ok {
						if r, err := fn(cmdArg[1:]...); err != nil {
							log4go.Error(err)
						} else if r != nil {
							fmt.Println(r)
						}
					} else {
						fmt.Println("not support : ", cmdArg[0])
					}
				}
			}
		}
	}()
	<-Stop
	return nil
}

func ConsoleCmd(ctx *cli.Context) error {
	start(ctx)
	return AttachCmd(ctx)
}
