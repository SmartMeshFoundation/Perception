package rpc

import "testing"

func TestFoo(t *testing.T) {
	t.Log("start")
	NewRPCServer(nil).Start()
}
