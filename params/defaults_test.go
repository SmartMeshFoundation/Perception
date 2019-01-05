package params

import (
	"testing"
	"time"
	"sync"
)

func TestHome(t *testing.T) {
	t.Log(home())
}

func TestWg(t *testing.T) {
	wg := new(sync.WaitGroup)
	go func() {
		wg.Wait()
		t.Log("wait...")
	}()
	wg.Add(1)
	t.Log("add...")
	wg.Done()
	t.Log("done...")
	<-time.After(time.Second * 3)
}
