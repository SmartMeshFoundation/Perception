package service

import "gx/ipfs/QmYaVXmXZNpWs6owQ1rk5VAiwNinkTh2cYZuYx1JDSactL/go-lightrpc/rpcserver"

var ServiceRegMap = make(map[string]rpcserver.ServiceReg)

func Registry(namespace, version string, service interface{}) {
	ServiceRegMap[namespace] = rpcserver.ServiceReg{
		Namespace: namespace,
		Version:   version,
		Service:   service,
	}
}
