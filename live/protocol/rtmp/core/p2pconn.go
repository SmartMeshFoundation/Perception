package core

import (
	"encoding/binary"
	"github.com/SmartMeshFoundation/Perception/live/utils/pool"
	inet "gx/ipfs/QmPtFaR7BWHLAjSwLh9kXcyrgTzDpuhcWLkx8ioa9RMYnx/go-libp2p-net"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"time"
)

type P2PConn struct {
	inet.Stream
	chunkSize           uint32
	remoteChunkSize     uint32
	windowAckSize       uint32
	remoteWindowAckSize uint32
	received            uint32
	ackReceived         uint32
	rw                  *ReadWriter
	pool                *pool.Pool
	chunks              map[uint32]ChunkStream
}

func NewP2PConn(s inet.Stream, bufferSize int) *P2PConn {
	return &P2PConn{
		Stream:              s,
		chunkSize:           128,
		remoteChunkSize:     128,
		windowAckSize:       2500000,
		remoteWindowAckSize: 2500000,
		pool:                pool.NewPool(),
		rw:                  NewReadWriter(s, bufferSize),
		chunks:              make(map[uint32]ChunkStream),
	}
}

func (conn *P2PConn) Read(c *ChunkStream) error {
	for {
		h, _ := conn.rw.ReadUintBE(1)
		// if err != nil {
		// 	log.Println("read from conn error: ", err)
		// 	return err
		// }
		format := h >> 6
		csid := h & 0x3f
		cs, ok := conn.chunks[csid]
		if !ok {
			cs = ChunkStream{}
			conn.chunks[csid] = cs
		}
		cs.tmpFromat = format
		cs.CSID = csid
		err := cs.readChunk(conn.rw, conn.remoteChunkSize, conn.pool)
		if err != nil {
			return err
		}
		conn.chunks[csid] = cs
		if cs.full() {
			*c = cs
			break
		}
	}

	conn.handleControlMsg(c)

	conn.ack(c.Length)

	return nil
}

func (conn *P2PConn) Write(c *ChunkStream) error {
	if c.TypeID == idSetChunkSize {
		conn.chunkSize = binary.BigEndian.Uint32(c.Data)
	}
	return c.writeChunk(conn.rw, int(conn.chunkSize))
}

func (conn *P2PConn) Flush() error {
	return conn.rw.Flush()
}

func (conn *P2PConn) Close() error {
	return conn.Stream.Close()
}

func (conn *P2PConn) RemoteAddr() peer.ID {
	return conn.Stream.Conn().RemotePeer()
}

func (conn *P2PConn) LocalAddr() peer.ID {
	return conn.Stream.Conn().LocalPeer()
}

func (conn *P2PConn) SetDeadline(t time.Time) error {
	return conn.Stream.SetDeadline(t)
}

func (conn *P2PConn) NewAck(size uint32) ChunkStream {
	return initControlMsg(idAck, 4, size)
}

func (conn *P2PConn) NewSetChunkSize(size uint32) ChunkStream {
	return initControlMsg(idSetChunkSize, 4, size)
}

func (conn *P2PConn) NewWindowAckSize(size uint32) ChunkStream {
	return initControlMsg(idWindowAckSize, 4, size)
}

func (conn *P2PConn) NewSetPeerBandwidth(size uint32) ChunkStream {
	ret := initControlMsg(idSetPeerBandwidth, 5, size)
	ret.Data[4] = 2
	return ret
}

func (conn *P2PConn) handleControlMsg(c *ChunkStream) {
	if c.TypeID == idSetChunkSize {
		conn.remoteChunkSize = binary.BigEndian.Uint32(c.Data)
	} else if c.TypeID == idWindowAckSize {
		conn.remoteWindowAckSize = binary.BigEndian.Uint32(c.Data)
	}
}

func (conn *P2PConn) ack(size uint32) {
	conn.received += uint32(size)
	conn.ackReceived += uint32(size)
	if conn.received >= 0xf0000000 {
		conn.received = 0
	}
	if conn.ackReceived >= conn.remoteWindowAckSize {
		cs := conn.NewAck(conn.ackReceived)
		cs.writeChunk(conn.rw, int(conn.chunkSize))
		conn.ackReceived = 0
	}
}


/*
   +------------------------------+-------------------------
   |     Event Type ( 2- bytes )  | Event Data
   +------------------------------+-------------------------
   Pay load for the ‘User Control Message’.
*/
func (conn *P2PConn) userControlMsg(eventType, buflen uint32) ChunkStream {
	var ret ChunkStream
	buflen += 2
	ret = ChunkStream{
		Format:   0,
		CSID:     2,
		TypeID:   4,
		StreamID: 1,
		Length:   buflen,
		Data:     make([]byte, buflen),
	}
	ret.Data[0] = byte(eventType >> 8 & 0xff)
	ret.Data[1] = byte(eventType & 0xff)
	return ret
}

func (conn *P2PConn) SetBegin() {
	ret := conn.userControlMsg(streamBegin, 4)
	for i := 0; i < 4; i++ {
		ret.Data[2+i] = byte(1 >> uint32((3-i)*8) & 0xff)
	}
	conn.Write(&ret)
}

func (conn *P2PConn) SetRecorded() {
	ret := conn.userControlMsg(streamIsRecorded, 4)
	for i := 0; i < 4; i++ {
		ret.Data[2+i] = byte(1 >> uint32((3-i)*8) & 0xff)
	}
	conn.Write(&ret)
}
