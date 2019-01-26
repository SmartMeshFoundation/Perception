package agents

import (
	"context"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/agents/pb"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBroadcastRecord_String(t *testing.T) {
	id, _ := peer.IDB58Decode("Qmduz9PhkP53UiTYUuNgp6JbWj69DWdbT9vWmw3BoH2sw3")
	b := newBroadcastRecord(id, agents_pb.AgentMessage_ADD_AS_TAB)
	t.Log(b)
}

func TestFlags(t *testing.T) {
	a := []string{
		0: "a",
		1: "b",
		2: "c",
	}
	t.Log(a)

	i := "a"
	switch i {
	case "a":
		fallthrough
	case "bb":
		fallthrough
	case "ccc":
		t.Log("ccc")
	default:
		t.Log("d")
	}
}

type Foo interface {
	Foobar()
}
type Bar struct{}

func (Bar) Foobar() {
	panic("implement me")
}

var _ Foo = (*Bar)(nil)

func TestFoo(t *testing.T) {

}

type A interface {
	AA()
}

type aImpl struct {
	b *b
}

type b struct {
	key string
	Val string
}

func (self *aImpl) AA() {
	fmt.Println("aaa -->", self.b.key, self.b.Val)
}

type Foobar struct {
	aa A
}

func TestA(t *testing.T) {
	foo := new(Foobar)
	a := new(aImpl)
	b := new(b)
	b.key = "hello "
	b.Val = "world"
	a.b = b
	foo.aa = a

	v := reflect.ValueOf(foo)
	v = v.Elem()
	aaV := v.FieldByName("aa")

	aaaV := aaV.Elem()

	aiV := aaaV.Convert(reflect.TypeOf(&aImpl{}))
	t.Log(aiV.Elem().Field(0).Elem().Field(0))
	t.Log(aiV.Elem().Field(0).Elem().Field(1))

	t.Log(aiV.Elem().Field(0).CanSet())
	t.Log(aiV.Elem().Field(0).Elem().Field(1).CanSet())

}

func TestV(t *testing.T) {
	s := `
HTTP/1.1 200
Content-Type: text/plain;charset=UTF-8
Content-Length: 4
Date: Fri, 21 Dec 2018 09:14:27 GMT

pong
`
	sr := strings.Contains(s, "HTTP/1.1 200")

	t.Log(sr)
}

func Test(t *testing.T) {
	for i := 0; i < 1000; i++ {
		r := rand.Intn(3)
		if r == 2 {
			t.Log(i, r)
		}
	}
}

func TestAsyncCtx(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	go func(c context.Context) {
		<-c.Done()
		t.Log("over", 0)
	}(ctx)
	go func(c context.Context) {
		<-c.Done()
		t.Log("over", 1)
	}(ctx)
	cancel()
	t.Log("cancel")
	<-time.After(3 * time.Second)
}

func TestList(t *testing.T) {
	l := []int{0, 1, 2, 3, 4}
	t.Log(len(l))
	t.Log(l[0:5])
}
