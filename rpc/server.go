package rpc

import (
	"fmt"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"github.com/SmartMeshFoundation/Perception/params"
	"github.com/SmartMeshFoundation/Perception/rpc/service"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmYaVXmXZNpWs6owQ1rk5VAiwNinkTh2cYZuYx1JDSactL/go-lightrpc/rpcserver"
	"net/http"
	"strconv"
	"strings"
)

type RPCServer struct {
	node types.Node
}

func NewRPCServer(node types.Node) *RPCServer {
	return &RPCServer{node}
}

func (self *RPCServer) Start() {
	port := params.HTTPPort
	rs := &rpcserver.Rpcserver{
		Port:       port,
		ServiceMap: service.ServiceRegMap,
		ServeMux:   http.NewServeMux(),
	}
	rs.ServeMux.HandleFunc("/getfile", self.handleHTTPGetfile)
	rs.ServeMux.HandleFunc("/cat/", PartialHandler)

	liveserver := self.node.GetLiveServer()
	if liveserver != nil {
		hFn := func(p string) (string, types.HttpHandlerFn) {
			f, err := liveserver.GetControlHandler(p)
			if err != nil {
				panic(err)
			}
			return p, f
		}
		log4go.Info("rpc_server enable liveserver : /live/state")
		rs.ServeMux.HandleFunc(hFn("/live/state"))
		log4go.Info("rpc_server enable liveserver : /live/pull")
		rs.ServeMux.HandleFunc(hFn("/live/pull"))
		log4go.Info("rpc_server enable liveserver : /live/push")
		rs.ServeMux.HandleFunc(hFn("/live/push"))
	}

	err := rs.StartServer()
	if err == nil {
		log4go.Info("RPC Service started. :%d", rs.Port)
	} else {
		log4go.Error("RPC Service started failed. :%d , err=%v", params.DefaultHTTPPort, err)
	}
}

/*
form
-----
filepath
targetid
rf
rt
*/
func (self *RPCServer) handleHTTPGetfile(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	var (
		filepaths, targetids, rfs, rts []string
		filepath, targetid             string
		rf, rt                         int64
		ok                             bool
		err                            error
	)
	for k, v := range r.Header {
		fmt.Println("head :::> ", k, "=", v)
	}

	//range= bytes=rf-rt
	headRange := r.Header.Get("Range")
	fmt.Println("========> range=", headRange)
	if headRange != "" {
		rf, rt, err = getRange(headRange)
		if err != nil {
			log4go.Error(err)
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}
	}

	if filepaths, ok = r.Form["filepath"]; !ok {
		w.WriteHeader(500)
		return
	}
	if targetids, ok = r.Form["targetid"]; !ok {
		w.WriteHeader(500)
		return
	}
	if rfs, ok = r.Form["rf"]; ok {
		rf, err = strconv.ParseInt(rfs[0], 10, 64)
		if err != nil {
			w.WriteHeader(500)
			return
		}
	}
	if rts, ok = r.Form["rt"]; ok {
		rt, err = strconv.ParseInt(rts[0], 10, 64)
		if err != nil {
			w.WriteHeader(500)
			return
		}
	}

	log4go.Info("fp = %s , tid = %s , range = [ %d - %d ]", filepath, targetid, rf, rt)
	filepath, targetid = filepaths[0], targetids[0]

	// ---->
	fse, err := self.node.GetStreamGenerater().GenFileStream(filepath, targetid, rf, rt)
	if err != nil {
		log4go.Error(err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	defer func() {
		if fse != nil && fse.S != nil {
			fse.S.Close()
		}
	}()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Add("ETag", fse.Hash)
	fsize := fse.Size
	if rf >= 0 && rt > 0 {
		fsize = rt - rf
		//Content-Range: bytes 0-10/3103
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rf, rt, fse.Size))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", rt-rf))
		w.Header().Add("partmd5", fse.PartHash)
		w.Header().Set("Accept-Ranges", "bytes")
	} else {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fse.Size))
	}
	log4go.Info("set_response_header = %v", w.Header())

	buff := make([]byte, 4096)
	total := int64(0)
	for total < fsize {
		t, err := fse.S.Read(buff)
		if err != nil {
			log4go.Error(err)
			return
		}
		if t <= 0 {
			log4go.Info("<- EOF ->")
			return
		}
		w.Write(buff[0:t])
		total += int64(t)
	}

}

//range= bytes=rf-rt
func getRange(headRange string) (int64, int64, error) {
	r1 := strings.Split(headRange, "=")
	if len(r1) != 2 {
		return 0, 0, errors.New("head_range_fail_format")
	}
	r2 := strings.Split(r1[1], "-")
	if len(r2) != 2 {
		return 0, 0, errors.New("head_range_fail_format")
	}
	rfs := strings.Trim(r2[0], " ")
	rts := strings.Trim(r2[1], " ")
	rf, err := strconv.ParseInt(rfs, 10, 64)
	if err != nil {
		return 0, 0, err
	}
	rt, err := strconv.ParseInt(rts, 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return rf, rt, nil
}
