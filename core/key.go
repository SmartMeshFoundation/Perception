package core

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/SmartMeshFoundation/Perception/params"
	ic "gx/ipfs/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"io/ioutil"
	"os"
	"path"
)

type MyID struct {
	ID  string `json:"id"`
	PRV string `json:"prv"`
}

func GenKey(dir string) (ic.PrivKey, error) {
	if dir == "" {
		dir = params.HomeDir
	}
	p := path.Join(dir, "myid")
	os.Remove(p)
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		panic(err) // oh no!
	}
	priv, pub, err := ic.GenerateKeyPairWithReader(ic.RSA, 2048, rand.Reader)
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		panic(err) // oh no!
	}
	prvb, _ := priv.Bytes()
	prvs := base64.StdEncoding.EncodeToString(prvb)
	id := pid.Pretty()
	myid := &MyID{id, prvs}
	jb, _ := json.Marshal(myid)
	n, err := f.WriteString(string(jb))
	fmt.Println(n, err)
	return priv, nil
}

func LoadKey(dir string) (ic.PrivKey, error) {
	if dir == "" {
		dir = params.HomeDir
	}
	p := path.Join(dir, "myid")
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	buf, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	myid := &MyID{}
	err = json.Unmarshal(buf, myid)
	if err != nil {
		return nil, err
	}
	pkb, err := base64.StdEncoding.DecodeString(myid.PRV)
	if err != nil {
		return nil, err
	}
	return ic.UnmarshalPrivateKey(pkb)
}
