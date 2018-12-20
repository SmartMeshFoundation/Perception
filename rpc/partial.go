package rpc

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

const (
	//TODO port 要用参数传进来
	apiHead = "http://localhost:45001/api/v0/cat/%s"
	apiGet  = "http://localhost:45001/api/v0/cat/%s?offset=%d&length=%d"
)

var (
	err     error
	res     []*http.Response
	sizeMap = make(map[string]int64)
)

func PartialHandler(w http.ResponseWriter, req *http.Request) {

	url := req.URL.String()

	reg := regexp.MustCompile(`/cat/([\w]+)`)

	hash := reg.FindStringSubmatch(url)[1]

	contentRange := req.Header.Get("range")

	var response *http.Response

	var start, end, contentLength int64
	if fileSize, ok := sizeMap[hash]; ok {
		start = getRange2(contentRange)
		end = fileSize - 1
		contentLength = fileSize - start
		response, err = http.Get(fmt.Sprintf(apiGet, hash, start, contentLength))
	} else {
		response, err = http.Get(fmt.Sprintf(apiHead, hash))
		total, _ := strconv.ParseInt(response.Header.Get("x-content-length"), 10, 64)
		end = total - 1
		contentLength = total
		sizeMap[hash] = total
	}
	if len(res) != 0 {
		for _, r := range res {
			if r != nil && r.Body != nil {
				r.Body.Close()
			}
		}
	}
	res = append(res, response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Add("Accept-Ranges", "bytes")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Range, X-Chunked-Output, X-Stream-Output")
	w.Header().Add("Access-Control-Allow-Methods", "GET")
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Expose-Headers", "Content-Range, X-Chunked-Output, X-Stream-Output")
	w.Header().Add("Cache-Control", "no-cache")
	//w.Header().Add("Cache-Control", "public, max-age=29030400, immutable")
	w.Header().Add("Content-Length", strconv.FormatInt(contentLength, 10))
	w.Header().Add("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, sizeMap[hash]))
	w.Header().Add("Content-Type", "application/octet-stream")
	//w.Header().Add("Last-Modified", "Thu, 01 Jan 1970 00:00:01 GMT")
	//w.Header().Add("Etag", hash)

	w.WriteHeader(http.StatusPartialContent)
	buffer := make([]byte, 1024*256)
	for {
		size, err := response.Body.Read(buffer)
		if err == nil {
			w.Write(buffer[:size])
		} else {
			if err == io.EOF {
				w.Write(buffer[:size])
			} else {
			}
			break
		}
	}
}

func getRange2(contentRange string) int64 {
	reg := regexp.MustCompile(`bytes=([0-9]*)-([0-9]*)`)

	ranges := reg.FindStringSubmatch(contentRange)

	start, _ := strconv.ParseInt(ranges[1], 10, 64)

	return start
}
