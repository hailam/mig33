// Package protocol implements the mig33 Java ME binary packet envelope.
//
// Field payloads carry no type discriminator. Callers choose an accessor from
// the opcode's schema. Numeric accessors read a big-endian prefix, as the
// original client does, and report false for missing or undersized payloads.
package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
	"unicode/utf16"
)

const (
	// HeaderSize is the marker, opcode, transaction, and body-length header.
	HeaderSize = 9
	// MaxFrameSize bounds the entire frame, including its nine-byte header.
	MaxFrameSize = 1 << 20
	// MaxBodySize is the largest accepted tagged-field body.
	MaxBodySize     = MaxFrameSize - HeaderSize
	fieldHeaderSize = 6
	marker          = 0x02
)

var (
	ErrInvalidMagic  = errors.New("protocol: invalid packet marker")
	ErrInvalidLength = errors.New("protocol: invalid length")
	ErrFrameTooLarge = errors.New("protocol: frame exceeds size limit")
	ErrNilPacket     = errors.New("protocol: nil packet")
)

// Packet is a decoded frame. Field tags, opcodes, and transactions are signed
// Java shorts. Duplicate field tags on the wire keep the final payload.
//
// Fields is intentionally exposed for schema-independent inspection. Callers
// must not mutate it concurrently with encoding or accessor calls.
type Packet struct {
	Opcode      int16
	Transaction int16
	Fields      map[int16][]byte
}

func New(op, txn int16) *Packet {
	return &Packet{Opcode: op, Transaction: txn, Fields: make(map[int16][]byte)}
}

func (p *Packet) SetText(tag int16, v string) *Packet {
	return p.set(tag, []byte(v))
}

func (p *Packet) SetByte(tag int16, v byte) *Packet {
	return p.set(tag, []byte{v})
}

func (p *Packet) SetShort(tag int16, v int16) *Packet {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(v))
	return p.set(tag, b)
}

func (p *Packet) SetInt(tag int16, v int32) *Packet {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(v))
	return p.set(tag, b)
}

func (p *Packet) SetLong(tag int16, v int64) *Packet {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return p.set(tag, b)
}

// SetBytes copies v so subsequent changes to the input cannot change a packet.
// A nil or empty payload is a present, zero-length field.
func (p *Packet) SetBytes(tag int16, v []byte) *Packet {
	return p.set(tag, bytes.Clone(v))
}

func (p *Packet) set(tag int16, v []byte) *Packet {
	if p.Fields == nil {
		p.Fields = make(map[int16][]byte)
	}
	p.Fields[tag] = v
	return p
}

func (p *Packet) field(tag int16, width int) ([]byte, bool) {
	if p == nil {
		return nil, false
	}
	v, ok := p.Fields[tag]
	return v, ok && len(v) >= width
}

// Text returns the raw UTF-8 payload as a Go string. The codec preserves bytes
// rather than applying schema-specific validation or character replacement.
func (p *Packet) Text(tag int16) (string, bool) {
	v, ok := p.field(tag, 0)
	return string(v), ok
}

func (p *Packet) Byte(tag int16) (byte, bool) {
	v, ok := p.field(tag, 1)
	if !ok {
		return 0, false
	}
	return v[0], true
}

func (p *Packet) Short(tag int16) (int16, bool) {
	v, ok := p.field(tag, 2)
	if !ok {
		return 0, false
	}
	return int16(binary.BigEndian.Uint16(v)), true
}

func (p *Packet) Int(tag int16) (int32, bool) {
	v, ok := p.field(tag, 4)
	if !ok {
		return 0, false
	}
	return int32(binary.BigEndian.Uint32(v)), true
}

func (p *Packet) Long(tag int16) (int64, bool) {
	v, ok := p.field(tag, 8)
	if !ok {
		return 0, false
	}
	return int64(binary.BigEndian.Uint64(v)), true
}

// Bytes returns a copy. Its boolean distinguishes absent and empty fields.
func (p *Packet) Bytes(tag int16) ([]byte, bool) {
	v, ok := p.field(tag, 0)
	return bytes.Clone(v), ok
}

// Decode reads exactly one frame without consuming a following frame. It
// returns io.EOF only when no header byte is available at the start of a frame.
// Truncated headers/bodies wrap io.ErrUnexpectedEOF. Invalid input never
// returns a partially decoded packet.
func Decode(r io.Reader) (*Packet, error) {
	var header [HeaderSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, fmt.Errorf("protocol: read header: %w", err)
	}
	if header[0] != marker {
		return nil, fmt.Errorf("%w: 0x%02x", ErrInvalidMagic, header[0])
	}
	size := binary.BigEndian.Uint32(header[5:9])
	if size > 0x7fffffff {
		return nil, fmt.Errorf("%w: negative body length", ErrInvalidLength)
	}
	if size > MaxBodySize {
		return nil, ErrFrameTooLarge
	}
	body := make([]byte, int(size))
	if _, err := io.ReadFull(r, body); err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return nil, fmt.Errorf("protocol: read body: %w", err)
	}
	p := New(int16(binary.BigEndian.Uint16(header[1:3])), int16(binary.BigEndian.Uint16(header[3:5])))
	for offset := 0; offset < len(body); {
		if len(body)-offset < fieldHeaderSize {
			return nil, fmt.Errorf("%w: incomplete field header at body offset %d", ErrInvalidLength, offset)
		}
		tag := int16(binary.BigEndian.Uint16(body[offset : offset+2]))
		length := binary.BigEndian.Uint32(body[offset+2 : offset+6])
		offset += fieldHeaderSize
		// This bound also rejects negative signed lengths, without integer
		// overflow or an allocation based on an untrusted field length.
		if uint64(length) > uint64(len(body)-offset) {
			return nil, fmt.Errorf("%w: field %d extends beyond body", ErrInvalidLength, tag)
		}
		end := offset + int(length)
		p.Fields[tag] = body[offset:end:end]
		offset = end
	}
	return p, nil
}

// Encode writes one complete frame with fields in ascending signed-tag order.
// It validates the size before writing any bytes. A failed writer can still
// receive a partial frame, as with other io.Writer APIs.
func Encode(w io.Writer, p *Packet) error {
	if p == nil {
		return ErrNilPacket
	}
	tags := make([]int, 0, len(p.Fields))
	bodySize := 0
	for tag, value := range p.Fields {
		if len(value) > MaxBodySize-fieldHeaderSize || bodySize > MaxBodySize-fieldHeaderSize-len(value) {
			return ErrFrameTooLarge
		}
		bodySize += fieldHeaderSize + len(value)
		tags = append(tags, int(tag))
	}
	sort.Ints(tags)
	frame := make([]byte, HeaderSize+bodySize)
	frame[0] = marker
	binary.BigEndian.PutUint16(frame[1:3], uint16(p.Opcode))
	binary.BigEndian.PutUint16(frame[3:5], uint16(p.Transaction))
	binary.BigEndian.PutUint32(frame[5:9], uint32(bodySize))
	offset := HeaderSize
	for _, tag := range tags {
		value := p.Fields[int16(tag)]
		binary.BigEndian.PutUint16(frame[offset:offset+2], uint16(int16(tag)))
		binary.BigEndian.PutUint32(frame[offset+2:offset+6], uint32(len(value)))
		copy(frame[offset+fieldHeaderSize:], value)
		offset += fieldHeaderSize + len(value)
	}
	n, err := w.Write(frame)
	if err != nil {
		return err
	}
	if n != len(frame) {
		return io.ErrShortWrite
	}
	return nil
}

// JavaHash returns Java String.hashCode() over UTF-16 code units, including
// surrogate pairs for supplementary characters. It does not lowercase input;
// the caller must apply the target client's password case-conversion rules.
func JavaHash(s string) int32 {
	var hash uint32
	for _, r := range s {
		if r > 0xffff {
			hi, lo := utf16.EncodeRune(r)
			hash = 31*hash + uint32(hi)
			hash = 31*hash + uint32(lo)
		} else {
			hash = 31*hash + uint32(r)
		}
	}
	return int32(hash)
}
