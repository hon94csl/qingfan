package ai

import (
	"context"
	"testing"
)

func TestUnknownCostIsNotZeroAndRetriesAreKept(t *testing.T) {
	provider := &staticProvider{err: ErrRateLimit}
	g := testGateway(t, provider, 2)
	token, err := g.Issue("shop-a", 5)
	if err != nil {
		t.Fatal(err)
	}
	call := Call{TaskID: "task", Kind: "script", Model: "test", PromptVersion: PromptVersion, Attempt: 1, Input: []byte("prompt")}
	if _, err := g.Do(context.Background(), token, call); err == nil {
		t.Fatal("expected rate limit")
	}
	row, err := g.lookup(token)
	if err != nil {
		t.Fatal(err)
	}
	id, err := g.latestEvent(row.Tenant, "rate_limited")
	if err != nil {
		t.Fatal(err)
	}
	micros, ok, state, err := g.EventCost(id)
	if err != nil {
		t.Fatal(err)
	}
	if ok || micros != 0 || state != "unknown" {
		t.Fatalf("missing supplier usage stored as %d (%s)", micros, state)
	}
	provider.err = nil
	provider.usage = Usage{Known: true, Micros: 11}
	provider.body = []byte(`{"ok":true}`)
	call.Attempt = 2
	if _, err := g.Do(context.Background(), token, call); err != nil {
		t.Fatal(err)
	}
	report, err := g.Costs(token)
	if err != nil {
		t.Fatal(err)
	}
	if report.KnownMicros != 11 || report.UnknownCalls != 1 || report.BillableCalls != 2 {
		t.Fatalf("%+v", report)
	}
}
