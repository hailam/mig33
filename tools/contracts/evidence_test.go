package main

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

// These checks compare the reviewed contract to independent original-client
// audit records. They deliberately do not derive expectations from the generator.
type observedTagView struct {
	Tag  int
	View string
}

type outgoingAuditEvidence struct {
	Operations []struct {
		Opcode   int
		Builders []struct {
			Fields []struct {
				Tag  int
				Type string
			}
		}
	}
}

type incomingAuditEvidence struct {
	Operations []struct {
		Opcode int
		Views  []struct {
			Tag           *int
			View          string
			PresenceReads []int `json:"presence_guard_offsets"`
			ValueReads    []int `json:"value_read_offsets"`
		} `json:"current_response_field_views"`
	}
	Annex []struct {
		Opcode int
		Fields []struct {
			Tag  int
			View string
		} `json:"response_fields"`
		Delegates []struct {
			Getters []struct {
				Tag  int
				View string
			} `json:"getter_sites"`
		}
	} `json:"reviewed_delegated_alias_and_dynamic_annex"`
}

func readEvidenceJSON(t *testing.T, name string, dst any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("../..", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("read evidence %s: %v", name, err)
	}
}

func evidenceContract(t *testing.T) (*contract, map[string]message) {
	t.Helper()
	c, err := loadContract("../..")
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]message, len(c.Messages))
	for _, m := range c.Messages {
		byID[m.ID] = m
	}
	return c, byID
}

func valueViewSet(m message) map[observedTagView]bool {
	result := map[observedTagView]bool{}
	for _, f := range m.Fields {
		for _, view := range f.Views {
			result[observedTagView{f.Tag, view}] = true
		}
	}
	return result
}

func presenceViewSet(m message) map[observedTagView]bool {
	result := map[observedTagView]bool{}
	for _, f := range m.Fields {
		for _, view := range f.PresenceViews {
			result[observedTagView{f.Tag, view}] = true
		}
	}
	return result
}

func requireEvidenceViews(t *testing.T, label string, got, want map[observedTagView]bool) {
	t.Helper()
	for view := range want {
		if !got[view] {
			t.Errorf("%s lost audited tag%d view%s", label, view.Tag, view.View)
		}
	}
	for view := range got {
		if !want[view] {
			t.Errorf("%s added unaudited tag%d view%s", label, view.Tag, view.View)
		}
	}
}

func TestOutgoingFieldsMatchOriginalBuilderEvidence(t *testing.T) {
	_, byID := evidenceContract(t)
	var audit outgoingAuditEvidence
	readEvidenceJSON(t, "research/applets/v4/docs/data/protocol-audit-outgoing.json", &audit)
	if len(audit.Operations) != 56 {
		t.Fatalf("audit coverage changed: got%d outgoing rows, want56", len(audit.Operations))
	}
	for _, operation := range audit.Operations {
		t.Run(strconv.Itoa(operation.Opcode), func(t *testing.T) {
			want := map[observedTagView]bool{}
			for _, builder := range operation.Builders {
				for _, f := range builder.Fields {
					want[observedTagView{f.Tag, f.Type}] = true
				}
			}
			m := byID["Client"+strconv.Itoa(operation.Opcode)]
			requireEvidenceViews(t, m.ID, valueViewSet(m), want)
			if len(presenceViewSet(m)) != 0 {
				t.Errorf("%s imports receiver presence reads into outgoing builder metadata", m.ID)
			}
		})
	}
}

func TestIncomingFieldsMatchCurrentResponseAndDelegatedEvidence(t *testing.T) {
	_, byID := evidenceContract(t)
	var audit incomingAuditEvidence
	readEvidenceJSON(t, "research/applets/v4/docs/data/protocol-audit-incoming.json", &audit)
	if len(audit.Operations) != 55 {
		t.Fatalf("audit coverage changed: got%d incoming rows, want55", len(audit.Operations))
	}
	for _, operation := range audit.Operations {
		t.Run(strconv.Itoa(operation.Opcode), func(t *testing.T) {
			values, presence := map[observedTagView]bool{}, map[observedTagView]bool{}
			for _, f := range operation.Views {
				if f.Tag == nil {
					continue // Computed tags are covered by the reviewed1002 rule below.
				}
				view := observedTagView{*f.Tag, f.View}
				if len(f.ValueReads) > 0 {
					values[view] = true
				}
				if len(f.PresenceReads) > 0 {
					presence[view] = true
				}
			}
			for _, annex := range audit.Annex {
				if annex.Opcode != operation.Opcode {
					continue
				}
				switch annex.Opcode {
				case 502:
					for _, f := range annex.Fields {
						values[observedTagView{f.Tag, f.View}] = true
					}
				case 927:
					for _, delegate := range annex.Delegates {
						for _, f := range delegate.Getters {
							values[observedTagView{f.Tag, f.View}] = true
						}
					}
				case 1000:
					// Reviewed bc login builder reads installed config1 as the revision.
					// Generic dynamic UI keys do not become invented concrete fields.
					values[observedTagView{1, "I"}] = true
				case 1002:
					// Reviewed loop reads dense triples; these are the first entry,
					// not an assertion that the resource has exactly three tags.
					for _, tag := range []int{1, 2, 3} {
						values[observedTagView{tag, "T"}] = true
					}
				}
			}
			m := byID["Server"+strconv.Itoa(operation.Opcode)]
			requireEvidenceViews(t, m.ID+" values", valueViewSet(m), values)
			requireEvidenceViews(t, m.ID+" presence", presenceViewSet(m), presence)
		})
	}
	for _, opcode := range []int{1000, 1002, 1004} {
		m := byID["Server"+strconv.Itoa(opcode)]
		if len(m.DynamicFields) == 0 || !m.OpenFields {
			t.Errorf("%s lost its open dynamic resource map", m.ID)
		}
	}
	// The icon helper reached from208 consumes another cached packet. In contrast,
	// the identical helper reached from927 consumes the current response.
	if got := valueViewSet(byID["Server208"]); !maps.Equal(got, map[observedTagView]bool{
		{1, "B"}: true, {2, "T"}: true, {3, "B"}: true, {4, "B"}: true, {5, "B"}: true,
	}) {
		t.Errorf("208 was polluted by cached icon fields: %v", got)
	}
}

func TestGatewayChannelsExcludeLocalJobsAndResourceDownloads(t *testing.T) {
	c, byID := evidenceContract(t)
	var outgoing struct{ Operations []struct{ Opcode int } }
	var incoming []struct{ Opcode int }
	readEvidenceJSON(t, "research/applets/v4/docs/data/protocol-outgoing.json", &outgoing)
	readEvidenceJSON(t, "research/applets/v4/docs/data/protocol-incoming.json", &incoming)
	requests, responses := map[string]bool{}, map[string]bool{}
	for _, row := range outgoing.Operations {
		id := "Client" + strconv.Itoa(row.Opcode)
		m := byID[id]
		if slices.Contains([]int{1001, 1003, 1005}, row.Opcode) {
			if m.Direction != "local" || !slices.Equal(m.Delivery, []string{"local"}) {
				t.Errorf("%s is a local HTTP job, not a gateway request", id)
			}
			continue
		}
		requests[id] = true
	}
	for _, row := range incoming {
		id := "Server" + strconv.Itoa(row.Opcode)
		m := byID[id]
		switch row.Opcode {
		case 1002, 1004:
			if m.Direction != "server-to-client" || !slices.Equal(m.Delivery, []string{"http-resource"}) {
				t.Errorf("%s is a separate HTTP resource result", id)
			}
		case 1005:
			if m.Direction != "local" || !slices.Equal(m.Delivery, []string{"local"}) {
				t.Errorf("%s is a locally synthesized raw HTTP result", id)
			}
		default:
			responses[id] = true
		}
	}
	if len(requests) != 53 || len(responses) != 52 || !responses["Server1000"] {
		t.Fatal("gateway evidence coverage changed; review transport boundaries")
	}
	doc := asyncAPI(c, nil)
	channels := doc["channels"].(object)
	operations := doc["operations"].(object)
	for _, test := range []struct {
		channel   string
		operation string
		action    string
		want      map[string]bool
	}{
		{"tcpRequests", "receiveTcpRequests", "receive", requests},
		{"httpRequests", "receiveHttpRequests", "receive", requests},
		{"tcpResponses", "sendTcpResponses", "send", responses},
		{"httpResponses", "sendHttpResponses", "send", responses},
	} {
		t.Run(test.channel, func(t *testing.T) {
			channel := channels[test.channel].(object)
			got := map[string]bool{}
			for id := range channel["messages"].(object) {
				got[id] = true
			}
			if !maps.Equal(got, test.want) {
				t.Errorf("gateway members differ from independent catalog/transport evidence: got%v want%v", got, test.want)
			}
			operation := operations[test.operation].(object)
			if operation["action"] != test.action {
				t.Errorf("operation direction must be %s from server perspective", test.action)
			}
		})
	}
}
