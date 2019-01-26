package agents

import (
	"fmt"
	"math/big"
	"testing"
	"time"
)

func TestReport(t *testing.T) {
	enableReport("ipfs")
	Record("ipfs", big.NewInt(100))
	// key=ipfs , start=20181225 , end=20181225
	r := Report("ipfs", "20181224", "20181225")
	t.Log(r)
}

func TestDateFormat(t *testing.T) {
	oneday, _ := time.ParseDuration("24h")
	s, err := time.Parse("20060102", "20181231")
	t.Log(err, s)
	e, err := time.Parse("20060102", "20190110")
	t.Log(err, e)
	for ; !s.Equal(e.Add(oneday)); s = s.Add(oneday) {
		t.Log(s.Format("20060102"),s.Before(e))
	}

	x := big.NewInt(30)
	t.Log(x.String()+" == 三十")
}

func TestThread(t *testing.T) {
	//wg := new(sync.WaitGroup)
	ch := make(chan string)
	valFn := func(n int) {
		defer t.Log("close thread : ", n)
		for c := range ch {
			t.Log(n, c)
		}
	}
	for i := 0; i < 3; i++ {
		go valFn(i)
	}

	for j := 0; j < 20; j ++ {
		ch <- fmt.Sprintf("task__%d", j)
	}

	close(ch)

	<-time.After(3 * time.Second)
}
