package utils

import (
	"github.com/SmartMeshFoundation/Perception/agents"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	"github.com/urfave/cli"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
)

var HOME_DIR, DATA_DIR, AGENTCLIENT, AGENTSERVER, NETWORK_ID string
var (
	LogLevel = []log4go.Level{log4go.ERROR, log4go.WARNING, log4go.INFO, log4go.DEBUG}
	App      *cli.App
	Node     types.Node
	Astab    *agents.Astable
	Stop     chan struct{}
	PPROF    bool

	Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "seed",
			Usage: "start as a bootnode rule",
		},
		cli.IntFlag{
			Name:  "port,p",
			Usage: "listen port",
			Value: params.DefaultPort,
		},
		cli.IntFlag{
			Name:  "rpcport",
			Usage: "http rpc listen port",
			Value: params.DefaultHTTPPort,
		},
		cli.StringFlag{
			Name:        "homedir",
			Usage:       "set home dir",
			Value:       params.HomeDir,
			Destination: &HOME_DIR,
		},
		cli.StringFlag{
			Name:        "netkey,n",
			Usage:       "set network key for private net, use 'genkey' subcommand to get a key",
			Destination: &NETWORK_ID,
		},
		cli.StringFlag{
			Name:        "datadir",
			Usage:       "set data dir , you can read and write this directories.",
			Destination: &DATA_DIR,
		},
		cli.IntFlag{
			Name:  "loglevel",
			Usage: "0:error , 1:warning , 2:info , 3:debug",
			Value: 2,
		},
		cli.BoolFlag{
			Name:        "pprof",
			Usage:       "record cpu、mem、block pprof info.",
			Destination: &PPROF,
		},

		cli.StringFlag{
			Name:        "agentclient,c",
			Usage:       "support web3rpc、web3ws、ipfsapi、ipfsgateway , format : name1:port1,name2:port2",
			Destination: &AGENTCLIENT,
		},
		cli.StringFlag{
			Name:        "agentserver,s",
			Usage:       "support web3rpc、web3ws、ipfsapi、ipfsgateway , format : name1:port1,name2:port2",
			Destination: &AGENTSERVER,
		},
	}
)
