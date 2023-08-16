package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

type GobCodec struct {
	conn io.ReadWriteCloser // Connection to client
	buf  *bufio.Writer      // Buffered writer for writing to conn
	dec  *gob.Decoder       // For reading header & body
	enc  *gob.Encoder       // For writing header & body
}

var _ Codec = (*GobCodec)(nil)

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}

func (g GobCodec) Close() error {
	return g.conn.Close()
}

func (g GobCodec) ReadHeader(header *Header) error {
	return g.dec.Decode(header)
}

func (g GobCodec) ReadBody(body interface{}) error {
	return g.dec.Decode(body)
}

func (g GobCodec) Write(header *Header, body interface{}) (err error) {
	defer func() {
		_ = g.buf.Flush()
		if err != nil {
			_ = g.conn.Close()
		}
	}()
	if err = g.enc.Encode(header); err != nil {
		log.Printf("rpc codec: gob error encoding header: %s", err)
		return
	}
	if err = g.enc.Encode(body); err != nil {
		log.Printf("rpc codec: gob error encoding body: %s", err)
		return
	}
	return nil

}
