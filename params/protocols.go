package params

import (
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
)

const (
	// protocols
	P_CHANNEL_FILE       = protocol.ID("/channel/file")
	P_CHANNEL_PING       = protocol.ID("/channel/ping")
	P_CHANNEL_BRIDGE     = protocol.ID("/channel/bridge")
	P_CHANNEL_AGENTS     = protocol.ID("/channel/agents")
	P_AGENT_WEB3_RPC     = protocol.ID("/agent/web3rpc")
	P_AGENT_WEB3_WS      = protocol.ID("/agent/web3ws")
	P_AGENT_IPFS_API     = protocol.ID("/agent/ipfsapi")
	P_AGENT_IPFS_GATEWAY = protocol.ID("/agent/ipfsgateway")
	P_AGENT_REST         = protocol.ID("/agent/rest")
)

const (
	BRIDGE_ACTION_PING = iota
	BRIDGE_ACTION_FILE
)
