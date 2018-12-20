# Perception
## Summary

>A P2P service that connects to other third-party services, such as IPFS, RSK, etc and we can use it on any device, including mobile devices, so that these devices can enjoy the services and functions of these third parties without running other IPFS nodes or public chain nodes.

## Build the source

>The `gx` will used for Perception's deps manage so before compiling, you need to know how to use `gx`.
                                                   
* [gx english documentation](https://github.com/whyrusleeping/gx)

* [gx 中文文档](https://www.jianshu.com/p/cfab2338f4a5)

### clone and compile

first , you need clone source to your `$GOPATH/src/github.com/SmartMeshFoundation` and use gx to install all deps.

```
#> cd $GOPATH/src/github.com/SmartMeshFoundation
#> git clone https://github.com/SmartMeshFoundation/Perception.git
#> cd Perception
#> gx install
```

__more platform compiled__

1. local : for dev and test

    `make local`

1. darwin-amd64 
    
    `make darwin`
    
1. windows-amd64
    
    `make windows`
    
1. linux-amd64

    `make linux`

1. android : deps `gomobile` `android-ndk` `android-sdk` 
    
    `make android`

1. ios : deps `gomobile` `xcode` 

    `make ios`





# Example:

## Run Perception

```bash
#> build/perception -h
NAME:
   build/perception - P2P network program

USAGE:
   perception [global options] command [command options] [arguments...]

VERSION:
   0.0.1-beta

AUTHOR:
   liangc <cc14514@icloud.com>

COMMANDS:
     version
     genkey
     console  一个简单的交互控制台，用来调试
     attach   连接本地已启动的进程
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --seed                         start as a bootnode rule
   --port value, -p value         listen port (default: 40001)
   --rpcport value                http rpc listen port (default: 40080)
   --homedir value                set home dir
   --netkey value, -n value       set network key for private net, use 'genkey' subcommand to get a key
   --datadir value                set data dir , you can read and write this directories.
   --loglevel value               0:error , 1:warning , 2:info , 3:debug (default: 2)
   --pprof                        record cpu、mem、block pprof info.
   --agentclient value, -c value  support web3rpc、web3ws、ipfsapi、ipfsgateway , format : name1:port1,name2:port2
   --agentserver value, -s value  support web3rpc、web3ws、ipfsapi、ipfsgateway , format : name1:port1,name2:port2
   --help, -h                     show help
   --version, -v                  print the version
```


## ipfs agent (ipfs light node)

* ipfs api default listen on tcp 5001 
    
* ipfs gateway default listen on tcp 8080 

* agent-server/agent-client on different computers and can be used in different networks.


### agent-server 

1. start ipfs :

    `ipfs daemon`

2. start perception as agent-server for ipfs api and gateway:

    `./perception -s "ipfsapi:5001,ipfsgateway:8080"`

### agent-client 

* start perception as agent-client for ipfs api and gateway:
    
    `./perception -c "ipfsapi:45001,ipfsgateway:48080"`

* ipfs api example:
  
    `curl http://localhost:45001/api/v0/swarm/peers`

* ipfs gateway example: 
    
    `curl http://localhost:48080/ipns/QmRWuyGyWT56oLxXPCmmbCvZFYnnU5P8uQgKxx4qhMoQ67`

## API

### getastab

__Get agent-server table , used to check if agent-server has been found.__

* request:

```
http://localhost:40080/api/?body={"sn":"sn-101","service":"funcs","method":"getastab"}
```

* response:

```
{
  "sn": "sn-101",
  "success": true,
  "entity": {
    "/agent/ipfsapi": [
      "Qmduz9PhkP53UiTYUuNgp6JbWj69DWdbT9vWmw3BoH2sw3"
    ],
    "/agent/ipfsgateway": [
      "Qmduz9PhkP53UiTYUuNgp6JbWj69DWdbT9vWmw3BoH2sw3"
    ],
    "/agent/web3rpc": [
      "Qmduz9PhkP53UiTYUuNgp6JbWj69DWdbT9vWmw3BoH2sw3"
    ],
    "/agent/web3ws": [
      "Qmduz9PhkP53UiTYUuNgp6JbWj69DWdbT9vWmw3BoH2sw3"
    ]
  }
}
```
  
