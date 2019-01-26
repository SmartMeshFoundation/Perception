package core

import (
	"fmt"
	"io"

	"time"

	"github.com/cc14514/superagent/live/utils/pio"
)


func (conn *P2PConn) HandshakeClient() (err error) {
	var random [(1 + 1536*2) * 2]byte

	C0C1C2 := random[:1536*2+1]
	C0 := C0C1C2[:1]
	C0C1 := C0C1C2[:1536+1]
	C2 := C0C1C2[1536+1:]

	S0S1S2 := random[1536*2+1:]

	C0[0] = 3
	// > C0C1
	conn.Stream.SetDeadline(time.Now().Add(timeout))
	if _, err = conn.rw.Write(C0C1); err != nil {
		return
	}
	conn.Stream.SetDeadline(time.Now().Add(timeout))
	if err = conn.rw.Flush(); err != nil {
		return
	}

	// < S0S1S2
	conn.Stream.SetDeadline(time.Now().Add(timeout))
	if _, err = io.ReadFull(conn.rw, S0S1S2); err != nil {
		return
	}

	S1 := S0S1S2[1 : 1536+1]
	if ver := pio.U32BE(S1[4:8]); ver != 0 {
		C2 = S1
	} else {
		C2 = S1
	}

	// > C2
	conn.Stream.SetDeadline(time.Now().Add(timeout))
	if _, err = conn.rw.Write(C2); err != nil {
		return
	}
	conn.Stream.SetDeadline(time.Time{})
	return
}

func (conn *P2PConn) HandshakeServer() (err error) {
	var random [(1 + 1536*2) * 2]byte

	C0C1C2 := random[:1536*2+1]
	C0 := C0C1C2[:1]
	C1 := C0C1C2[1 : 1536+1]
	C0C1 := C0C1C2[:1536+1]
	C2 := C0C1C2[1536+1:]

	S0S1S2 := random[1536*2+1:]
	S0 := S0S1S2[:1]
	S1 := S0S1S2[1 : 1536+1]
	S0S1 := S0S1S2[:1536+1]
	S2 := S0S1S2[1536+1:]

	// < C0C1
	conn.Stream.SetDeadline(time.Now().Add(timeout))
	if _, err = io.ReadFull(conn.rw, C0C1); err != nil {
		return
	}
	conn.Stream.SetDeadline(time.Now().Add(timeout))
	if C0[0] != 3 {
		err = fmt.Errorf("rtmp: handshake version=%d invalid", C0[0])
		return
	}

	S0[0] = 3

	clitime := pio.U32BE(C1[0:4])
	srvtime := clitime
	srvver := uint32(0x0d0e0a0d)
	cliver := pio.U32BE(C1[4:8])

	if cliver != 0 {
		var ok bool
		var digest []byte
		if ok, digest = hsParse1(C1, hsClientPartialKey, hsServerFullKey); !ok {
			err = fmt.Errorf("rtmp: handshake server: C1 invalid")
			return
		}
		hsCreate01(S0S1, srvtime, srvver, hsServerPartialKey)
		hsCreate2(S2, digest)
	} else {
		copy(S1, C2)
		copy(S2, C1)
	}

	// > S0S1S2
	conn.Stream.SetDeadline(time.Now().Add(timeout))
	if _, err = conn.rw.Write(S0S1S2); err != nil {
		return
	}
	conn.Stream.SetDeadline(time.Now().Add(timeout))
	if err = conn.rw.Flush(); err != nil {
		return
	}

	// < C2
	conn.Stream.SetDeadline(time.Now().Add(timeout))
	if _, err = io.ReadFull(conn.rw, C2); err != nil {
		return
	}
	conn.Stream.SetDeadline(time.Time{})
	return
}
