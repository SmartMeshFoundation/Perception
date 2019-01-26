package service

import "gx/ipfs/QmVMCXmZjzL3jiMcapzDvLgsZ4EFP1CFuVGHsWLVBFB6ed/go-lightrpc/rpcserver"

var ServiceRegMap = make(map[string]rpcserver.ServiceReg)

func Registry(namespace, version string, service interface{}) {
	ServiceRegMap[namespace] = rpcserver.ServiceReg{
		Namespace: namespace,
		Version:   version,
		Service:   service,
	}
}

