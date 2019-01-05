package agents

import (
	"math/big"
	"testing"
)

func TestReport(t *testing.T) {
	enableReport("ipfs")
	Record("ipfs", big.NewInt(100))
	// key=ipfs , start=20181225 , end=20181225
	r := Report("ipfs", "20181224", "20181225")
	t.Log(r)
}
