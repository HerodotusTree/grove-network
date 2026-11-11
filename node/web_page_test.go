package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Stage B (node web page): the node's declared identity lives in the
// genesis config file and /status reports it, so the page the node
// serves can state the node's class from the node's own mouth. The
// account view is untouched - STATUS-01's shape stands, these fields
// are additive.
func TestStatusReportsDeclaredNodeClass(t *testing.T) {
	const genesis = `{"kind":"genesis","network":"test","node_class":"personal","node_name":"Grove Node — test"}`
	s := newTestServerGenesis(t, genesis)

	rec := httptest.NewRecorder()
	s.handleStatus(rec, httptest.NewRequest(http.MethodGet, "/status?account=deadbeef", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["node_class"] != "personal" {
		t.Fatalf("node_class: got %v, want personal", m["node_class"])
	}
	if m["node_name"] != "Grove Node — test" {
		t.Fatalf("node_name: got %v", m["node_name"])
	}
	// The pre-existing account fields ride along unchanged.
	if m["balance"] != float64(0) || m["domain"] == "" || m["height"] != float64(1) {
		t.Fatalf("account view disturbed: %v", m)
	}
}

// A genesis config that declares nothing reports nothing: the class
// is never assumed or defaulted, so the page shows Not declared.
func TestStatusNodeClassUndeclaredByDefault(t *testing.T) {
	s := newTestServer(t) // testGenesis declares no class or name

	rec := httptest.NewRecorder()
	s.handleStatus(rec, httptest.NewRequest(http.MethodGet, "/status?account=deadbeef", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["node_class"] != "" || m["node_name"] != "" {
		t.Fatalf("undeclared node must report empty identity: %v", m)
	}
}

// The genesis payload the ledger stores is the operator's bytes,
// class fields and all: declaring an identity moves the domain like
// any other config difference - nodes on one network share one file.
func TestDeclaredGenesisStillBinds(t *testing.T) {
	s := newTestServerGenesis(t, `{"kind":"genesis","network":"test","node_class":"business"}`)
	if s.nodeClass != "business" {
		t.Fatalf("server did not keep the declaration: %q", s.nodeClass)
	}
	if s.v.Domain == "" || s.v.Log.Len() != 1 {
		t.Fatal("genesis with identity fields must still write entry 0 and bind the domain")
	}
}
