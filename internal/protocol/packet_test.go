package protocol

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/iotest"
)

// This is an unchanged copy of research/applets/v4/docs/data/protocol-fixtures.json, produced
// by the original JAR's ay/bj serializers, not by this Go implementation.
//
//go:embed testdata/original_v4_packets.json
var originalFixtures []byte

type fixture struct {
	Name        string `json:"name"`
	Opcode      int16  `json:"opcode"`
	Transaction int16  `json:"transaction"`
	Fields      []struct {
		Tag   int16           `json:"tag"`
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	} `json:"fields"`
	Hex string `json:"original_java_hex"`
}

func fixtures(t testing.TB) []fixture {
	t.Helper()
	var catalog struct {
		Packets []fixture `json:"packets"`
	}
	if err := json.Unmarshal(originalFixtures, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Packets) != 9 {
		t.Fatalf("expected all 9 original v4 fixtures, got %d", len(catalog.Packets))
	}
	return catalog.Packets
}

func unhex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fixturePacket(t testing.TB, f fixture) *Packet {
	t.Helper()
	p := New(f.Opcode, f.Transaction)
	for _, field := range f.Fields {
		var value interface{}
		switch field.Type {
		case "T", "X":
			value = new(string)
		case "TA":
			value = new([]string)
		default:
			value = new(int64)
		}
		if err := json.Unmarshal(field.Value, value); err != nil {
			t.Fatal(err)
		}
		switch field.Type {
		case "B":
			p.SetByte(field.Tag, byte(*value.(*int64)))
		case "S":
			p.SetShort(field.Tag, int16(*value.(*int64)))
		case "I":
			p.SetInt(field.Tag, int32(*value.(*int64)))
		case "L":
			p.SetLong(field.Tag, *value.(*int64))
		case "T":
			p.SetText(field.Tag, *value.(*string))
		case "X":
			p.SetBytes(field.Tag, unhex(t, *value.(*string)))
		case "TA":
			p.SetBytes(field.Tag, []byte(strings.Join(*value.(*[]string), "\x00")))
		default:
			t.Fatalf("unknown fixture type %q", field.Type)
		}
	}
	return p
}

// sortedOriginal preserves each original raw TLV, changing only its position.
// Java Hashtable ordering differs from the deterministic order we emit.
func sortedOriginal(t testing.TB, raw []byte) []byte {
	t.Helper()
	var fields [][]byte
	for offset := HeaderSize; offset < len(raw); {
		if len(raw)-offset < 6 {
			t.Fatal("invalid golden field header")
		}
		end := offset + 6 + int(binary.BigEndian.Uint32(raw[offset+2:offset+6]))
		if end > len(raw) {
			t.Fatal("invalid golden field length")
		}
		fields = append(fields, raw[offset:end])
		offset = end
	}
	sort.Slice(fields, func(i, j int) bool {
		return int16(binary.BigEndian.Uint16(fields[i])) < int16(binary.BigEndian.Uint16(fields[j]))
	})
	return append(bytes.Clone(raw[:HeaderSize]), bytes.Join(fields, nil)...)
}

func TestOriginalJavaPackets(t *testing.T) {
	for _, f := range fixtures(t) {
		t.Run(f.Name, func(t *testing.T) {
			raw := unhex(t, f.Hex)
			want := fixturePacket(t, f)
			got, err := Decode(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("original decode = %#v; want %#v", got, want)
			}
			for name, packet := range map[string]*Packet{"decoded": got, "constructed": want} {
				t.Run(name, func(t *testing.T) {
					var out bytes.Buffer
					if err := Encode(&out, packet); err != nil {
						t.Fatal(err)
					}
					if expected := sortedOriginal(t, raw); !bytes.Equal(out.Bytes(), expected) {
						t.Fatalf("wire bytes = %x; want original TLVs reordered = %x", out.Bytes(), expected)
					}
					roundTrip, err := Decode(&out)
					if err != nil || !reflect.DeepEqual(roundTrip, want) {
						t.Fatalf("round trip = %#v, %v; want %#v", roundTrip, err, want)
					}
				})
			}
		})
	}
}

func TestConcatenatedAndFragmentedStream(t *testing.T) {
	var stream []byte
	all := fixtures(t)
	for _, f := range all {
		stream = append(stream, unhex(t, f.Hex)...)
	}
	// A TCP reader may return one byte at a time and EOF alongside its last byte.
	r := iotest.DataErrReader(iotest.OneByteReader(bytes.NewReader(stream)))
	for _, f := range all {
		got, err := Decode(r)
		if err != nil || !reflect.DeepEqual(got, fixturePacket(t, f)) {
			t.Fatalf("stream packet %s = %#v, %v", f.Name, got, err)
		}
	}
	if p, err := Decode(r); p != nil || !errors.Is(err, io.EOF) {
		t.Fatalf("clean stream end = %#v, %v; want EOF", p, err)
	}
}

func TestDuplicateTagsKeepLastIncludingEmpty(t *testing.T) {
	for _, test := range []struct{ hex, want string }{
		{"0200010001000000120001000000036f6c640001000000036e6577", "new"},
		{"02000100010000000f0001000000036f6c64000100000000", ""},
	} {
		p, err := Decode(bytes.NewReader(unhex(t, test.hex)))
		if err != nil {
			t.Fatal(err)
		}
		got, ok := p.Text(1)
		if !ok || got != test.want || len(p.Fields) != 1 {
			t.Fatalf("last duplicate = %q, %v, fields=%d", got, ok, len(p.Fields))
		}
	}
	if got, _ := New(602, 1).SetText(1, "new-avatar").SetText(1, "old-avatar").Text(1); got != "old-avatar" {
		t.Fatalf("setter replacement = %q", got)
	}
}

func TestDeterministicSignedTagOrder(t *testing.T) {
	p := New(-1, -32768).SetByte(0, 2).SetByte(-1, 1).SetBytes(-32768, nil)
	want := unhex(t, "02ffff800000000014800000000000ffff000000010100000000000102")
	for range 20 {
		var out bytes.Buffer
		if err := Encode(&out, p); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out.Bytes(), want) {
			t.Fatalf("encoding = %x; want %x", out.Bytes(), want)
		}
	}
}

func TestAccessorsAndByteOwnership(t *testing.T) {
	p := &Packet{}
	input := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	p.SetBytes(1, input).SetText(2, "").SetByte(3, 255).
		SetShort(4, -32768).SetInt(5, -2147483648).SetLong(6, -9223372036854775808)
	input[0] = 99
	out, ok := p.Bytes(1)
	if !ok || out[0] != 1 {
		t.Fatal("SetBytes retained caller-owned memory")
	}
	out[0] = 88
	if v, _ := p.Byte(1); v != 1 {
		t.Fatal("Bytes exposed mutable packet memory")
	}
	if v, ok := p.Short(1); !ok || v != 0x0102 {
		t.Fatalf("short prefix = %x, %v", v, ok)
	}
	if v, ok := p.Int(1); !ok || v != 0x01020304 {
		t.Fatalf("int prefix = %x, %v", v, ok)
	}
	if v, ok := p.Long(1); !ok || v != 0x0102030405060708 {
		t.Fatalf("long prefix = %x, %v", v, ok)
	}
	if v, ok := p.Text(2); !ok || v != "" {
		t.Fatal("empty text must remain present")
	}
	if _, ok := p.Byte(2); ok {
		t.Fatal("empty bytes cannot be read as a byte")
	}
	if _, ok := p.Short(3); ok {
		t.Fatal("one-byte field cannot be read as short")
	}
	if _, ok := p.Int(4); ok {
		t.Fatal("two-byte field cannot be read as int")
	}
	if _, ok := p.Long(5); ok {
		t.Fatal("four-byte field cannot be read as long")
	}
	if v, ok := p.Byte(3); !ok || v != 255 {
		t.Fatalf("unsigned byte = %d, %v", v, ok)
	}
	if v, ok := p.Short(4); !ok || v != -32768 {
		t.Fatalf("signed short = %d, %v", v, ok)
	}
	if v, ok := p.Int(5); !ok || v != -2147483648 {
		t.Fatalf("signed int = %d, %v", v, ok)
	}
	if v, ok := p.Long(6); !ok || v != -9223372036854775808 {
		t.Fatalf("signed long = %d, %v", v, ok)
	}
	for _, absent := range []*Packet{p, nil} {
		if _, ok := absent.Text(99); ok {
			t.Fatal("missing text reported present")
		}
		if _, ok := absent.Bytes(99); ok {
			t.Fatal("missing bytes reported present")
		}
		if _, ok := absent.Byte(99); ok {
			t.Fatal("missing byte reported present")
		}
		if _, ok := absent.Short(99); ok {
			t.Fatal("missing short reported present")
		}
		if _, ok := absent.Int(99); ok {
			t.Fatal("missing int reported present")
		}
		if _, ok := absent.Long(99); ok {
			t.Fatal("missing long reported present")
		}
	}
}

func TestDecodeRejectsMalformedFrames(t *testing.T) {
	for _, test := range []struct {
		name string
		hex  string
		want error
	}{
		{"empty", "", io.EOF},
		{"partial_header", "020001", io.ErrUnexpectedEOF},
		{"invalid_marker", "030001000100000000", ErrInvalidMagic},
		{"negative_body", "0200010001ffffffff", ErrInvalidLength},
		{"max_signed_body", "02000100017fffffff", ErrFrameTooLarge},
		{"frame_over_limit", "0200010001000ffff8", ErrFrameTooLarge},
		{"missing_body", "020001000100000006", io.ErrUnexpectedEOF},
		{"partial_body", "0200010001000000060001", io.ErrUnexpectedEOF},
		{"short_field_header", "0200010001000000050001000000", ErrInvalidLength},
		{"negative_field", "0200010001000000060001ffffffff", ErrInvalidLength},
		{"overflow_field", "02000100010000000600017fffffff", ErrInvalidLength},
		{"field_outside_frame", "02000100010000000700010000000261", ErrInvalidLength},
		{"trailing_body_byte", "02000100010000000700010000000000", ErrInvalidLength},
	} {
		t.Run(test.name, func(t *testing.T) {
			p, err := Decode(bytes.NewReader(unhex(t, test.hex)))
			if p != nil || !errors.Is(err, test.want) {
				t.Fatalf("Decode = %#v, %v; want nil, %v", p, err, test.want)
			}
		})
	}
	// A malicious field cannot consume bytes belonging to a following frame.
	broken := unhex(t, "020001000100000006000100000064")
	following := unhex(t, "020002000200000000")
	r := bytes.NewReader(append(broken, following...))
	if _, err := Decode(r); !errors.Is(err, ErrInvalidLength) {
		t.Fatal(err)
	}
	if r.Len() != len(following) {
		t.Fatalf("bad field consumed following frame: remaining=%d", r.Len())
	}
	if p, err := Decode(r); err != nil || p.Opcode != 2 || p.Transaction != 2 {
		t.Fatalf("following frame = %#v, %v", p, err)
	}
}

func TestFrameSizeBoundary(t *testing.T) {
	p := New(500, 1).SetBytes(8, bytes.Repeat([]byte{0xa5}, MaxBodySize-6))
	var out bytes.Buffer
	if err := Encode(&out, p); err != nil || out.Len() != MaxFrameSize {
		t.Fatalf("maximum frame encoded length=%d, err=%v", out.Len(), err)
	}
	got, err := Decode(&out)
	if err != nil || !bytes.Equal(got.Fields[8], p.Fields[8]) {
		t.Fatalf("maximum frame decode failed: %v", err)
	}
	for _, oversized := range []*Packet{
		New(500, 1).SetBytes(8, make([]byte, MaxBodySize-5)),
		New(500, 1).SetBytes(8, make([]byte, MaxBodySize/2)).SetBytes(9, make([]byte, MaxBodySize/2)),
	} {
		out.Reset()
		if err := Encode(&out, oversized); !errors.Is(err, ErrFrameTooLarge) || out.Len() != 0 {
			t.Fatalf("oversized packet wrote %d bytes, err=%v", out.Len(), err)
		}
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

type controlledWriter struct {
	n   int
	err error
}

func (w controlledWriter) Write([]byte) (int, error) { return w.n, w.err }

func TestIOErrors(t *testing.T) {
	want := errors.New("test transport failure")
	if _, err := Decode(errorReader{want}); !errors.Is(err, want) {
		t.Fatalf("reader error lost: %v", err)
	}
	header := unhex(t, "020001000100000006")
	if _, err := Decode(io.MultiReader(bytes.NewReader(header), errorReader{want})); !errors.Is(err, want) {
		t.Fatalf("body reader error lost: %v", err)
	}
	if err := Encode(controlledWriter{3, want}, New(2, 1)); !errors.Is(err, want) {
		t.Fatalf("writer error lost: %v", err)
	}
	if err := Encode(controlledWriter{3, nil}, New(2, 1)); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write accepted: %v", err)
	}
	if err := Encode(io.Discard, nil); !errors.Is(err, ErrNilPacket) {
		t.Fatalf("nil packet accepted: %v", err)
	}
}

func TestJavaHashUTF16(t *testing.T) {
	for _, test := range []struct {
		input string
		want  int32
	}{
		{"", 0},
		{"abc", 96354},
		{"hello", 99162322},
		{"😀", 1772899},
		{"Hello ✓ 😀", 684918846},
		{"𐐷\x00é", 1701529535},
		{"challengepassword", -564190210},
		{"polygenelubricants", -2147483648},
		{"Aa", 2112},
		{"BB", 2112},
	} {
		if got := JavaHash(test.input); got != test.want {
			t.Errorf("JavaHash(%q)=%d; want %d", test.input, got, test.want)
		}
	}
}

func FuzzDecode(f *testing.F) {
	for _, original := range fixtures(f) {
		f.Add(unhex(f, original.Hex))
	}
	f.Add([]byte{})
	f.Add(unhex(f, "0200010001000000060001ffffffff"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		r := bytes.NewReader(raw)
		p, err := Decode(r)
		if err != nil {
			if p != nil {
				t.Fatal("error returned a partial packet")
			}
			return
		}
		if consumed := len(raw) - r.Len(); consumed < HeaderSize || consumed > MaxFrameSize {
			t.Fatalf("accepted invalid frame size %d", consumed)
		}
		var encoded bytes.Buffer
		if err := Encode(&encoded, p); err != nil {
			t.Fatalf("decoded packet cannot be encoded: %v", err)
		}
		roundTrip, err := Decode(&encoded)
		if err != nil || !reflect.DeepEqual(p, roundTrip) {
			t.Fatalf("round trip failed: %v", err)
		}
	})
}
