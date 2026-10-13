package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type staticProvider struct {
	mu      sync.Mutex
	body    []byte
	usage   Usage
	err     error
	calls   int
	hold    chan struct{}
	started chan struct{}
}

func (s *staticProvider) Name() string { return "test" }

func (s *staticProvider) Complete(ctx context.Context, kind string, input []byte) ([]byte, Usage, error) {
	s.mu.Lock()
	s.calls++
	if s.started != nil && s.calls == 1 {
		close(s.started)
		s.started = nil
	}
	hold := s.hold
	s.mu.Unlock()
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
			return nil, Usage{}, ErrTimeout
		}
	}
	if s.err != nil {
		return nil, s.usage, s.err
	}
	return s.body, s.usage, nil
}

func (s *staticProvider) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func testGateway(t *testing.T, provider Provider, limit int) *Gateway {
	t.Helper()
	g, err := Open(t.TempDir()+"/usage.sqlite", provider, limit)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { g.Close() })
	return g
}

func TestUnauthorizedRevokedAndTenant(t *testing.T) {
	g := testGateway(t, &staticProvider{body: []byte(`{"ok":true}`), usage: Usage{Known: true, Micros: 3}}, 2)
	if _, err := g.Do(context.Background(), "", Call{TaskID: "t", Kind: "script", Model: "test", PromptVersion: PromptVersion, Attempt: 1, Input: []byte("a")}); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	token, err := g.Issue("shop-a", 4)
	if err != nil {
		t.Fatal(err)
	}
	other, err := g.Issue("shop-b", 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Do(context.Background(), token, Call{Tenant: "shop-b", TaskID: "t", Kind: "script", Model: "test", PromptVersion: PromptVersion, Attempt: 1, Input: []byte("a")}); !errors.Is(err, ErrTenant) {
		t.Fatal(err)
	}
	if _, err := g.Do(context.Background(), token, Call{TaskID: "t", Kind: "script", Model: "test", PromptVersion: PromptVersion, Attempt: 1, Input: []byte("a")}); err != nil {
		t.Fatal(err)
	}
	if err := g.Revoke(token); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Do(context.Background(), token, Call{TaskID: "t", Kind: "script", Model: "test", PromptVersion: PromptVersion, Attempt: 1, Input: []byte("b")}); !errors.Is(err, ErrRevoked) {
		t.Fatal(err)
	}
	report, err := g.Costs(other)
	if err != nil {
		t.Fatal(err)
	}
	if report.BillableCalls != 0 || report.KnownMicros != 0 {
		t.Fatalf("tenant leak %+v", report)
	}
}

func TestConcurrencyQuotaAndCache(t *testing.T) {
	hold := make(chan struct{})
	provider := &staticProvider{body: []byte(`{"ok":true}`), usage: Usage{Known: true, Micros: 5}, hold: hold, started: make(chan struct{})}
	g := testGateway(t, provider, 1)
	token, err := g.Issue("shop-a", 1)
	if err != nil {
		t.Fatal(err)
	}
	call := Call{TaskID: "t", Kind: "script", Model: "test", PromptVersion: PromptVersion, Attempt: 1, Input: []byte("same")}
	done := make(chan error, 1)
	go func() {
		_, err := g.Do(context.Background(), token, call)
		done <- err
	}()
	<-provider.started
	if _, err := g.Do(context.Background(), token, Call{TaskID: "t", Kind: "script", Model: "test", PromptVersion: PromptVersion, Attempt: 1, Input: []byte("other")}); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	close(hold)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := g.Do(context.Background(), token, call); err != nil {
		t.Fatal(err)
	}
	if provider.Calls() != 1 {
		t.Fatalf("cache missed, calls=%d", provider.Calls())
	}
	report, err := g.Costs(token)
	if err != nil {
		t.Fatal(err)
	}
	if report.BillableCalls != 1 || report.CacheHits != 1 || report.KnownMicros != 5 {
		t.Fatalf("%+v", report)
	}
	if _, err := g.Do(context.Background(), token, Call{TaskID: "t", Kind: "script", Model: "test", PromptVersion: PromptVersion, Attempt: 1, Input: []byte("fresh")}); !errors.Is(err, ErrQuota) {
		t.Fatal(err)
	}
}

func TestRetryTimeoutAndLogRedaction(t *testing.T) {
	provider := &staticProvider{err: ErrTimeout, usage: Usage{Known: true, Micros: 7}}
	g := testGateway(t, provider, 2)
	var logBuf bytes.Buffer
	g.SetLogger(log.New(&logBuf, "", 0))
	token, err := g.Issue("shop-a", 6)
	if err != nil {
		t.Fatal(err)
	}
	call := Call{TaskID: "task-1", Kind: "tts", Model: "test", PromptVersion: PromptVersion, Voice: "ida", Attempt: 1, Input: []byte("audio")}
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		call.Attempt = attempt
		if _, err := g.Do(context.Background(), token, call); !errors.Is(err, ErrTimeout) {
			t.Fatal(err)
		}
	}
	call.Attempt = MaxAttempts + 1
	if _, err := g.Do(context.Background(), token, call); !errors.Is(err, ErrRetry) {
		t.Fatal(err)
	}
	report, err := g.Costs(token)
	if err != nil {
		t.Fatal(err)
	}
	if report.KnownMicros != 21 || report.BillableCalls != 3 {
		t.Fatalf("%+v", report)
	}
	if strings.Contains(logBuf.String(), token) {
		t.Fatalf("log kept the token: %s", logBuf.String())
	}
}

func TestHTTPAuthAndSupplierKey(t *testing.T) {
	const key = "super-secret-provider-key"
	supplier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+key {
			http.Error(w, "missing", http.StatusUnauthorized)
			return
		}
		switch r.Header.Get("X-Qingfan-Kind") {
		case "tts":
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		case "rate":
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"output":{"clips":[]},"usage_known":false,"cost_micros":0}`)
	}))
	defer supplier.Close()
	provider := &HTTPProvider{URL: supplier.URL, Key: secret(key)}
	if strings.Contains(fmt.Sprintf("%+v %#v %s", provider, provider, provider.Key), key) {
		t.Fatal("provider formatting exposed the key")
	}
	g := testGateway(t, provider, 2)
	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)
	g.SetLogger(logger)
	srv := httptest.NewServer(Routes(g, "bootstrap-secret", logger))
	defer srv.Close()
	res, err := http.Post(srv.URL+"/v1/scripts", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", res.StatusCode)
	}
	issue, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/tokens", strings.NewReader(`{"tenant_id":"shop-a","quota":3}`))
	if err != nil {
		t.Fatal(err)
	}
	issue.Header.Set("X-Qingfan-Bootstrap", "bootstrap-secret")
	issued, err := http.DefaultClient.Do(issue)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(issued.Body)
	issued.Body.Close()
	if issued.StatusCode != http.StatusOK || strings.Contains(logBuf.String(), "bootstrap-secret") {
		t.Fatalf("issue %d log %s body %s", issued.StatusCode, logBuf.String(), raw)
	}
	var token struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &token); err != nil || token.Token == "" {
		t.Fatal(err, string(raw))
	}
	if strings.Contains(logBuf.String(), token.Token) || strings.Contains(logBuf.String(), key) {
		t.Fatalf("log leaked secrets: %s", logBuf.String())
	}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/scripts", strings.NewReader(`{"tenant_id":"shop-b","task_id":"t","facts":[{"id":"fact-1","text":"瓶盖"}],"clips":[]}`))
	req.Header.Set("Authorization", "Bearer "+token.Token)
	cross, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	cross.Body.Close()
	if cross.StatusCode != http.StatusForbidden {
		t.Fatalf("cross tenant %d", cross.StatusCode)
	}
	if _, _, err := provider.Complete(context.Background(), "tts", []byte(`{}`)); !errors.Is(err, ErrInvalidAudio) {
		t.Fatal(err)
	}
	if _, _, err := provider.Complete(context.Background(), "rate", []byte(`{}`)); !errors.Is(err, ErrRateLimit) {
		t.Fatal(err)
	}
	if _, err := g.Do(context.Background(), token.Token, Call{TaskID: "t", Kind: "analyze", Model: "http", PromptVersion: PromptVersion, Attempt: 1, Input: []byte("frames")}); err != nil {
		t.Fatal(err)
	}
	row, err := g.lookup(token.Token)
	if err != nil {
		t.Fatal(err)
	}
	id, err := g.latestEvent(row.Tenant, "succeeded")
	if err != nil {
		t.Fatal(err)
	}
	micros, ok, state, err := g.EventCost(id)
	if err != nil {
		t.Fatal(err)
	}
	if ok || micros != 0 || state != "unknown" {
		t.Fatalf("unknown cost stored as %d ok=%v state=%s", micros, ok, state)
	}
	if err := RequirePublicTLS(true, "", ""); err == nil {
		t.Fatal("public listen without a certificate")
	}
	if err := RequirePublicTLS(false, "", ""); err != nil {
		t.Fatal(err)
	}
}
