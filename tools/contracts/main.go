// Command contracts generates the recovered 4.62 contract without requiring
// Node, Java, or the original applet. Node is used separately for standards validation.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/hailam/mig33/internal/protocol"
)

const sourcePath = "api/mig33-v4.json"

// Canonical decimal spellings of signed Java short tags, including unknown
// future tags. Schema openness must not permit nonnumeric/out-of-range keys.
const tagPattern = "^(?:0|[1-9][0-9]{0,3}|[12][0-9]{4}|3[01][0-9]{3}|32[0-6][0-9]{2}|327[0-5][0-9]|3276[0-7]|-(?:[1-9][0-9]{0,3}|[12][0-9]{4}|3[01][0-9]{3}|32[0-6][0-9]{2}|327[0-5][0-9]|3276[0-8]))$"

type evidence struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256,omitempty"`
	Pointer string `json:"pointer,omitempty"`
	Note    string `json:"note,omitempty"`
}

type field struct {
	Tag           int        `json:"tag"`
	Views         []string   `json:"views"`
	Description   string     `json:"description"`
	Presence      string     `json:"presence"`
	PresenceViews []string   `json:"presence_views,omitempty"`
	Evidence      []evidence `json:"evidence,omitempty"`
}

type message struct {
	ID            string            `json:"id"`
	Opcode        int               `json:"opcode"`
	Direction     string            `json:"direction"`
	Delivery      []string          `json:"delivery"`
	Summary       string            `json:"summary"`
	Description   string            `json:"description"`
	Evidence      []evidence        `json:"evidence"`
	Fields        []field           `json:"fields"`
	OpenFields    bool              `json:"open_fields"`
	Notes         []string          `json:"notes"`
	DynamicFields json.RawMessage   `json:"dynamic_fields,omitempty"`
	ReplyOpcodes  []int             `json:"reply_opcodes,omitempty"`
	Variants      []json.RawMessage `json:"variants,omitempty"`
}

type contract struct {
	Profile           string     `json:"profile"`
	Version           string     `json:"version"`
	OriginalJARSHA256 string     `json:"original_jar_sha256"`
	Evidence          []evidence `json:"evidence"`
	Messages          []message  `json:"messages"`
}

type object = map[string]any

func main() {
	check := flag.Bool("check", false, "verify generated files without modifying them")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/contracts [--check]")
		os.Exit(2)
	}
	if err := run(".", *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(root string, check bool) error {
	c, err := loadContract(root)
	if err != nil {
		return err
	}
	outputs, err := generate(root, c)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(outputs))
	for path := range outputs {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		full := filepath.Join(root, path)
		if check {
			got, err := os.ReadFile(full)
			if err != nil || !bytes.Equal(got, outputs[path]) {
				return fmt.Errorf("%s is missing or stale; run sh scripts/generate-contracts.sh", path)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, outputs[path], 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("Contract: %d reviewed definitions; %d generated artifacts current\n", len(c.Messages), len(outputs))
	return nil
}

func loadContract(root string) (*contract, error) {
	raw, err := os.ReadFile(filepath.Join(root, sourcePath))
	if err != nil {
		return nil, err
	}
	var c contract
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", sourcePath, err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, errors.New("contract must contain exactly one JSON object")
	}
	if err := validateContract(root, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func validateContract(root string, c *contract) error {
	if c.Profile != "j2me-4.62" || c.Version == "" || c.OriginalJARSHA256 != "dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c" {
		return errors.New("contract profile/version/original JAR identity is invalid")
	}
	sources := map[string][]byte{}
	for _, e := range c.Evidence {
		if !filepath.IsLocal(e.Path) || e.SHA256 == "" || sources[e.Path] != nil {
			return fmt.Errorf("invalid or duplicate evidence path/hash: %s", e.Path)
		}
		raw, err := os.ReadFile(filepath.Join(root, e.Path))
		if err != nil {
			return err
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != e.SHA256 {
			return fmt.Errorf("evidence changed: %s; review its contract definitions before updating the hash", e.Path)
		}
		sources[e.Path] = raw
	}
	ids := map[string]bool{}
	for _, m := range c.Messages {
		if ids[m.ID] || m.ID == "" || !signedShort(m.Opcode) {
			return fmt.Errorf("duplicate/invalid message: %s", m.ID)
		}
		ids[m.ID] = true
		if m.ID != "Client"+strconv.Itoa(m.Opcode) && m.ID != "Server"+strconv.Itoa(m.Opcode) {
			return fmt.Errorf("message ID must preserve catalog side and opcode: %s", m.ID)
		}
		if !slices.Contains([]string{"client-to-server", "server-to-client", "local"}, m.Direction) || m.Summary == "" || !m.OpenFields {
			return fmt.Errorf("invalid direction/summary or closed raw fields: %s", m.ID)
		}
		if len(m.Delivery) == 0 {
			return fmt.Errorf("missing delivery classification: %s", m.ID)
		}
		for _, delivery := range m.Delivery {
			if !slices.Contains([]string{"tcp", "http-tunnel", "http-resource", "local"}, delivery) {
				return fmt.Errorf("unknown delivery %q on %s", delivery, m.ID)
			}
		}
		if slices.Contains(m.Delivery, "tcp") != slices.Contains(m.Delivery, "http-tunnel") {
			return fmt.Errorf("both gateway transports must be retained: %s", m.ID)
		}
		if (m.Direction == "local") != slices.Equal(m.Delivery, []string{"local"}) {
			return fmt.Errorf("local message cannot be advertised as a gateway operation: %s", m.ID)
		}
		if err := checkEvidence(sources, m.Evidence); err != nil {
			return fmt.Errorf("%s: %w", m.ID, err)
		}
		tags := map[int]bool{}
		for _, f := range m.Fields {
			if !signedShort(f.Tag) || tags[f.Tag] || f.Description == "" {
				return fmt.Errorf("%s: duplicate/invalid field %d", m.ID, f.Tag)
			}
			tags[f.Tag] = true
			if !slices.Contains([]string{"observed", "conditional", "always"}, f.Presence) {
				return fmt.Errorf("%s/%d: invalid presence", m.ID, f.Tag)
			}
			for _, views := range [][]string{f.Views, f.PresenceViews} {
				seen := map[string]bool{}
				for _, view := range views {
					if !slices.Contains([]string{"B", "S", "I", "L", "T", "X", "TA"}, view) || seen[view] {
						return fmt.Errorf("%s/%d: duplicate/invalid accessor %q", m.ID, f.Tag, view)
					}
					seen[view] = true
				}
			}
			if err := checkEvidence(sources, f.Evidence); err != nil {
				return fmt.Errorf("%s/%d: %w", m.ID, f.Tag, err)
			}
		}
	}
	return checkCatalogCoverage(sources, ids)
}

func signedShort(n int) bool { return n >= -32768 && n <= 32767 }

func checkEvidence(sources map[string][]byte, references []evidence) error {
	if len(references) == 0 {
		return errors.New("missing evidence references")
	}
	for _, e := range references {
		raw, ok := sources[e.Path]
		if !ok {
			return fmt.Errorf("unhashed evidence: %s", e.Path)
		}
		if e.Pointer == "" {
			continue
		}
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("JSON pointer on non-JSON evidence: %s", e.Path)
		}
		if !strings.HasPrefix(e.Pointer, "/") {
			return fmt.Errorf("invalid JSON pointer %s", e.Pointer)
		}
		for _, token := range strings.Split(e.Pointer[1:], "/") {
			token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
			switch v := value.(type) {
			case map[string]any:
				value, ok = v[token]
			case []any:
				index, err := strconv.Atoi(token)
				ok = err == nil && index >= 0 && index < len(v)
				if ok {
					value = v[index]
				}
			default:
				ok = false
			}
			if !ok {
				return fmt.Errorf("unresolved evidence pointer: %s#%s", e.Path, e.Pointer)
			}
		}
	}
	return nil
}

func checkCatalogCoverage(sources map[string][]byte, ids map[string]bool) error {
	type row struct{ Opcode int }
	var outgoing struct{ Operations []row }
	var incoming []row
	if err := json.Unmarshal(sources["research/applets/v4/docs/data/protocol-outgoing.json"], &outgoing); err != nil {
		return err
	}
	if err := json.Unmarshal(sources["research/applets/v4/docs/data/protocol-incoming.json"], &incoming); err != nil {
		return err
	}
	want := map[string]bool{}
	for _, r := range outgoing.Operations {
		want["Client"+strconv.Itoa(r.Opcode)] = true
	}
	for _, r := range incoming {
		want["Server"+strconv.Itoa(r.Opcode)] = true
	}
	if len(want) != len(ids) {
		return errors.New("contract does not cover exactly the outgoing and incoming catalogs")
	}
	for id := range want {
		if !ids[id] {
			return fmt.Errorf("catalog message is missing: %s", id)
		}
	}
	return nil
}

func generate(root string, c *contract) (map[string][]byte, error) {
	examples, err := fixtureExamples(root)
	if err != nil {
		return nil, err
	}
	doc := asyncAPI(c, examples)
	// Normalize through JSON so struct tags and RawMessage annotations retain
	// exactly the canonical source spelling in YAML.
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var normalized object
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return nil, err
	}
	var yamlOut bytes.Buffer
	yamlOut.WriteString("# Generated by sh scripts/generate-contracts.sh. DO NOT EDIT.\n")
	encoder := yaml.NewEncoder(&yamlOut)
	encoder.SetIndent(2)
	if err := encoder.Encode(normalized); err != nil {
		return nil, err
	}
	coverage, err := json.MarshalIndent(coverageReport(c), "", "  ")
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		"api/asyncapi.yaml":           yamlOut.Bytes(),
		"api/generated/mig33-v4.ts":   typeScript(c),
		"api/generated/coverage.json": append(coverage, '\n'),
	}, nil
}

func ref(path string) object { return object{"$ref": path} }

func asyncAPI(c *contract, examples map[int][]object) object {
	messages, schemas := object{}, object{}
	schemas["RawFieldHex"] = object{
		"type": "string", "pattern": "^(?:[0-9a-f]{2})*$",
		"description": "Lowercase hexadecimal projection of the exact field bytes. Empty string is present-empty; an absent key is absent. Hex is not sent over the network.",
	}
	for _, m := range c.Messages {
		props := object{}
		for _, f := range m.Fields {
			props[strconv.Itoa(f.Tag)] = object{
				"type": "string", "pattern": "^(?:[0-9a-f]{2})*$",
				"description": f.Description, "x-mig33-field": f,
			}
		}
		schemas[m.ID+"Packet"] = object{
			"type": "object", "additionalProperties": false,
			"required": []string{"opcode", "transaction", "fields"},
			"properties": object{
				"opcode":      object{"type": "integer", "enum": []int{m.Opcode}},
				"transaction": object{"type": "integer", "minimum": -32768, "maximum": 32767},
				"fields": object{
					"type": "object", "properties": props,
					"patternProperties":    object{tagPattern: ref("#/components/schemas/RawFieldHex")},
					"additionalProperties": false,
					"description":          "Map from signed-16-bit decimal tag to raw field hex. Unknown tags are preserved. No field-requiredness or historical server validation is inferred from client accessors.",
					"x-mig33-tag-range":    []int{-32768, 32767},
				},
			},
			"description": "Decoded inspection projection of one binary frame, not a JSON request or response.",
		}
		definition := object{
			"name": m.ID, "title": m.Summary, "summary": m.Summary,
			"description":            m.Description + "\n\n" + strings.Join(m.Notes, "\n"),
			"contentType":            "application/octet-stream",
			"payload":                ref("#/components/schemas/" + m.ID + "Packet"),
			"x-mig33-observation":    m,
			"x-mig33-representation": "decoded-raw-field-hex; documentation only",
		}
		if m.Direction != "local" {
			definition["correlationId"] = object{
				"location":    "$message.payload#/transaction",
				"description": "Transaction field in the decoded projection. Only matching pending flows correlate; pushes and resource downloads need not have a pending request. This does not imply uniqueness across sessions, exactly-once delivery, or one reply per request.",
			}
		}
		if values := examples[m.Opcode]; len(values) > 0 {
			definition["examples"] = values
		}
		messages[m.ID] = definition
	}
	channels, operations := object{}, object{}
	for _, transport := range []string{"tcp", "http-tunnel"} {
		for _, direction := range []string{"client-to-server", "server-to-client"} {
			id, action := "tcpRequests", "receive"
			if transport == "http-tunnel" {
				id = "httpRequests"
			}
			if direction == "server-to-client" {
				id = strings.ReplaceAll(id, "Requests", "Responses")
				action = "send"
			}
			members := object{}
			selected := []object{}
			for _, m := range c.Messages {
				if m.Direction == direction && slices.Contains(m.Delivery, transport) {
					members[m.ID] = ref("#/components/messages/" + m.ID)
					selected = append(selected, ref("#/channels/"+id+"/messages/"+m.ID))
				}
			}
			description := "Logical direction on one bidirectional raw TCP byte stream. These channel names are documentation identifiers, not socket addresses or additional endpoints. Buffer fragmented and concatenated frames."
			server := "localTCP"
			if transport == "http-tunnel" {
				description = "Logical packet direction over the original binary HTTP tunnel. POST sends frames; POST responses and immediate GET polls return zero or more concatenated frames. A packet is not an entire HTTP exchange. See openapi.yaml for actual routes, image/gif MIME, SID routing and HTTP errors."
				server = "localHTTP"
			}
			channels[id] = object{
				"address": nil, "description": description, "messages": members,
				"servers":           []object{ref("#/servers/" + server)},
				"x-mig33-direction": direction,
			}
			operations[action+strings.ToUpper(id[:1])+id[1:]] = object{
				"action": action, "channel": ref("#/channels/" + id), "messages": selected,
				"summary": "Compatibility target: " + direction + " over " + transport,
			}
		}
	}
	return object{
		"asyncapi": "3.0.0", "id": "urn:mig33:compatibility:j2me:4.62",
		"info": object{
			"title": "mig33 J2ME 4.62 recovered compatibility target", "version": c.Version,
			"description": "Original-client observations, described from the target server's perspective. This is a compatibility target, not a claim that the Go reconstruction implements every operation. The version identifies this document, not a new network API. Payload schemas are raw-field inspection projections; actual traffic is binary. Local jobs and direct resource results are components only. The 3.08 profile is separate.",
		},
		"servers": object{
			"localTCP":  object{"host": "127.0.0.1:9119", "protocol": "tcp", "description": "Local reconstruction address; configurable. Original defaults and fallback behavior are recorded in the source endpoint catalog."},
			"localHTTP": object{"host": "127.0.0.1:8080", "protocol": "http", "description": "Local reconstruction address; original base/session paths are in openapi.yaml. No new endpoints."},
		},
		"channels": channels, "operations": operations,
		"components":            object{"messages": messages, "schemas": schemas},
		"x-mig33-source":        object{"path": sourcePath, "original_jar_sha256": c.OriginalJARSHA256, "evidence": c.Evidence},
		"x-mig33-http-contract": "openapi.yaml",
		"x-mig33-wire": object{
			"byte_order": "big-endian", "marker": 2, "header_bytes": 9,
			"frame":              "marker:u8, opcode:i16, transaction:i16, bodyLength:i32, body",
			"field":              "tag:i16, length:i32, rawPayload:length bytes",
			"type_discriminator": false, "duplicate_tags": "last payload wins",
			"field_order":    "arbitrary; original Hashtable order is not normative",
			"unknown_fields": "preserve", "unknown_opcodes": "outside observed coverage; original dispatcher falls through",
			"accessor_views": object{
				"B": "signed byte prefix", "S": "signed 16-bit prefix", "I": "signed 32-bit prefix",
				"L": "signed 64-bit prefix; use bigint, never a JavaScript number",
				"T": "raw UTF-8 text, no writeUTF length prefix", "X": "uninterpreted bytes",
				"TA": "UTF-8 strings separated by NUL with no final separator; original decoder can lose the final element",
			},
			"absence":       "absent and present-empty differ; original empty-byte accessor throws, undersized numeric accessors return null",
			"length_policy": "original parser accepts some malformed lengths; this contract describes valid frames and does not require reproducing unsafe parsing",
			"documentation": "../research/applets/v4/docs/protocol.md",
		},
	}
}

func fixtureExamples(root string) (map[int][]object, error) {
	raw, err := os.ReadFile(filepath.Join(root, "internal/protocol/testdata/original_v4_packets.json"))
	if err != nil {
		return nil, err
	}
	var fixtures struct {
		Packets []struct {
			Name string `json:"name"`
			Hex  string `json:"original_java_hex"`
		} `json:"packets"`
	}
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		return nil, err
	}
	result := map[int][]object{}
	for _, f := range fixtures.Packets {
		wire, err := hex.DecodeString(f.Hex)
		if err != nil {
			return nil, err
		}
		reader := bytes.NewReader(wire)
		packet, err := protocol.Decode(reader)
		if err != nil || reader.Len() != 0 {
			return nil, fmt.Errorf("invalid original serializer fixture %s: %v", f.Name, err)
		}
		fields := object{}
		for tag, value := range packet.Fields {
			fields[strconv.Itoa(int(tag))] = hex.EncodeToString(value)
		}
		result[int(packet.Opcode)] = append(result[int(packet.Opcode)], object{
			"name": f.Name, "summary": "Original serializer with synthetic values; wire example, not a captured historical service exchange.",
			"payload": object{"opcode": packet.Opcode, "transaction": packet.Transaction, "fields": fields},
		})
	}
	return result, nil
}

func typeScript(c *contract) []byte {
	var out strings.Builder
	out.WriteString("// Code generated by sh scripts/generate-contracts.sh. DO NOT EDIT.\n")
	out.WriteString("// Inspection types and accessor metadata; no network, binary codec, or session implementation.\n\n")
	out.WriteString("export interface PrimitiveViews { B: number; S: number; I: number; L: bigint; T: string; X: Uint8Array; TA: readonly string[]; }\n")
	out.WriteString("export type AccessorView = keyof PrimitiveViews;\n")
	out.WriteString("/** Actual in-memory raw fields; preserve unknown tags and absent versus present-empty. */\n")
	out.WriteString("export interface RawPacket<Opcode extends number = number> { readonly opcode: Opcode; readonly transaction: number; readonly fields: ReadonlyMap<number, Uint8Array>; }\n")
	out.WriteString("/** JSON inspection projection used by AsyncAPI examples; never transmitted as JSON. */\n")
	out.WriteString("export interface PacketProjection<Opcode extends number = number> { readonly opcode: Opcode; readonly transaction: number; readonly fields: Readonly<Record<string, string>>; }\n\n")
	out.WriteString("export interface MessageOpcodes {\n")
	for _, m := range c.Messages {
		fmt.Fprintf(&out, "  %s: %d;\n", m.ID, m.Opcode)
	}
	out.WriteString("}\nexport type MessageID = keyof MessageOpcodes;\n")
	out.WriteString("export type PacketFor<M extends MessageID> = RawPacket<MessageOpcodes[M]>;\n\n")
	out.WriteString("/** Only observed value views. Presence tests remain metadata; getters do not imply required fields. */\n")
	out.WriteString("export interface FieldViewNames {\n")
	for _, m := range c.Messages {
		fmt.Fprintf(&out, "  %s: {", m.ID)
		for _, f := range m.Fields {
			if len(f.Views) == 0 {
				continue
			}
			views := make([]string, len(f.Views))
			for i, view := range f.Views {
				views[i] = strconv.Quote(view)
			}
			fmt.Fprintf(&out, " %s: %s;", strconv.Quote(strconv.Itoa(f.Tag)), strings.Join(views, " | "))
		}
		out.WriteString(" };\n")
	}
	out.WriteString("}\n")
	out.WriteString("export type ObservedFieldViews<M extends MessageID> = { readonly [Tag in keyof FieldViewNames[M]]?: { readonly [View in FieldViewNames[M][Tag] & AccessorView]?: PrimitiveViews[View] } };\n\n")
	out.WriteString("export const messageDescriptors = {\n")
	for _, m := range c.Messages {
		fields := []object{}
		for _, f := range m.Fields {
			fields = append(fields, object{"tag": f.Tag, "views": f.Views, "presenceViews": nonNil(f.PresenceViews), "presence": f.Presence})
		}
		descriptor, _ := json.Marshal(object{
			"opcode": m.Opcode, "direction": m.Direction, "delivery": m.Delivery,
			"summary": m.Summary, "fields": fields, "openFields": true,
		})
		fmt.Fprintf(&out, "  %s: %s,\n", m.ID, descriptor)
	}
	out.WriteString("} as const;\n\n")
	for _, group := range []struct{ Name, Direction, Delivery string }{
		{"GatewayRequest", "client-to-server", "tcp"},
		{"GatewayResponse", "server-to-client", "tcp"},
		{"ResourceResult", "server-to-client", "http-resource"},
		{"LocalJob", "local", "local"},
	} {
		names := []string{}
		for _, m := range c.Messages {
			if m.Direction == group.Direction && slices.Contains(m.Delivery, group.Delivery) {
				names = append(names, strconv.Quote(m.ID))
			}
		}
		fmt.Fprintf(&out, "export type %sID = %s;\n", group.Name, strings.Join(names, " | "))
	}
	return []byte(out.String())
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func coverageReport(c *contract) object {
	rows := []object{}
	counts := map[string]int{}
	for _, m := range c.Messages {
		for _, delivery := range m.Delivery {
			counts[delivery+"/"+m.Direction]++
		}
		rows = append(rows, object{
			"id": m.ID, "opcode": m.Opcode, "delivery": m.Delivery,
			"field_count": len(m.Fields), "dynamic_fields": len(m.DynamicFields) > 0,
			"historical_server_validation": "not established", "runtime_implementation": "not asserted",
		})
	}
	return object{
		"generated_from": sourcePath, "profile": c.Profile,
		"definitions": len(c.Messages), "counts": counts, "messages": rows,
		"scope": "Coverage of reviewed original-client observations. Not a count of implemented Go features or complete historical server schemas.",
	}
}
