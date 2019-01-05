package core

import (
	"encoding/json"
	"fmt"
	"gopkg.in/karalabe/cookiejar.v2/collections/set"
	"math/rand"
	"os"
	"testing"
	"time"
)

func TestGenKey(t *testing.T) {
	prv1, _ := GenKey("")
	prv2, _ := LoadKey("")
	b1, _ := prv1.Bytes()
	b2, _ := prv2.Bytes()
	t.Log(string(b1) == string(b2))
}

func TestList(t *testing.T) {
	ss := []string{"aaa", "bbb", "ccc"}
	bb, err := json.Marshal(ss)
	t.Log(err, bb, ss)
	ss2 := make([]string, 0)
	err = json.Unmarshal(bb, &ss2)
	t.Log(err, ss2)

}

func TestRandID(t *testing.T) {
	for j := 0; j < 100; j++ {

		s := set.New()
		for i := 0; i < 100; i++ {
			a := rand.Intn(100)
			s.Insert(a)
		}
		fmt.Println(j, s.Size(), 100-s.Size())
	}
}

func TestFileRW(t *testing.T) {
	p := "/tmp/a.txt"
	go func() {
		f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			t.Error(err)
			panic(err)
		}
		for i := 0; i < 60; i++ {
			f.WriteString(fmt.Sprintf("hello world , %d \n", i))
			<-time.After(time.Millisecond*100)
		}
	}()

	<-time.After(time.Second * 1)

	r, err := os.OpenFile(p, os.O_RDONLY, 0444)
	//r, err := os.OpenFile(p, os.O_SYNC|os.O_RDONLY, 0444)
	if err != nil {
		t.Error(err)
		panic(err)
	}
	buf := make([]byte, 1024)
	for {
		c, err := r.Read(buf)
		if err != nil {
			fmt.Println(err)
			break
		}
		fmt.Println(string(buf[:c]))
	}
	<-time.After(time.Second * 30)

}
