package params

import (
	"fmt"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmW7Ump7YyBMr712Ta3iEVh3ZYcfVvJaPryfbCnyE826b4/go-libp2p-interface-pnet"
	"gx/ipfs/QmZaQ3K9PRd5sYYoG1xbTGPtd3N7TYiKBRmcBUTsx8HVET/go-libp2p-pnet"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	DefaultHTTPHost = "localhost" // Default host interface for the HTTP RPC server
	DefaultHTTPPort = 40080       // Default TCP port for the HTTP RPC server
	DefaultWSHost   = "localhost" // Default host interface for the websocket RPC server
	DefaultPort     = 40001

	BootstrapInterval = 30 // second

	AgentServerBroadcastInterval = 30 // second
)

var (
	NetworkID = "492133e95f196e8915a5c8b5f7a70777cea31606b0e20ff2e31f8dbceec83706"
	HomeDir   = defaultHomeDir()
	DataDir   = fmt.Sprintf("%s/data", HomeDir)
	HTTPPort  = DefaultHTTPPort
)

func defaultHomeDir() string {
	home := home()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "perception")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "Perception")
		} else {
			return filepath.Join(home, ".perception")
		}
	}
	return ""
}

func home() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

func SetHomeDir(homedir string) {
	HomeDir = homedir
	DataDir = fmt.Sprintf("%s/data", HomeDir)
}
func SetDataDir(datadir string) {
	DataDir = datadir
}
func SetNetworkID(nid string) {
	NetworkID = nid
}

func NewProtector() (ipnet.Protector, error) {
	if NetworkID == "" {
		return nil, errors.New("protector disable.")
	}
	tmp := `/key/swarm/psk/1.0.0/
/base16/
%s`
	key := fmt.Sprintf(tmp, NetworkID)
	r := strings.NewReader(key)
	return pnet.NewProtector(r)
}
