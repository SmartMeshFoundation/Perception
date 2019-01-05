package core

import (
	"testing"
	"context"
	"encoding/hex"
)

func TestLoopBootstrap(t *testing.T) {
	loop_bootstrap(nil)
}

func ctxValue(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, "foobar", "hello world")
	return ctx
}

func TestCtx(t *testing.T) {
	ctx := context.Background()
	ctx = ctxValue(ctx)
	t.Log(ctx.Value("foobar"))
}

func TestHashEncode(t *testing.T) {
	buf := make([]byte, 16)
	s := hex.EncodeToString(buf)
	t.Log("s=", s, s == "00000000000000000000000000000000")
}

func TestAbc(t *testing.T) {
	m := make(map[string]int)
	m["a"] = 1
	m["b"] = 1
	i := len(m)
	m["c"] = 1
	m["c"] = 2
	j := len(m)
	t.Log(len(m),i,j)
}
