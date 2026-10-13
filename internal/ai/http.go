package ai

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// secret redacts a supplier key in fmt output.
type secret string

func (secret) String() string   { return "[redacted]" }
func (secret) GoString() string { return "[redacted]" }

// HTTPProvider posts prompts to one supplier. The key is a header, never a log field.
type HTTPProvider struct {
	URL    string
	Key    secret
	Client *http.Client
}

func (p *HTTPProvider) Name() string { return "http" }

func (p *HTTPProvider) Complete(ctx context.Context, kind string, input []byte) ([]byte, Usage, error) {
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(input))
	if err != nil {
		return nil, Usage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+string(p.Key))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Qingfan-Kind", kind)
	res, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, Usage{}, ErrTimeout
		}
		return nil, Usage{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, Usage{}, err
	}
	switch res.StatusCode {
	case http.StatusTooManyRequests:
		return nil, Usage{}, ErrRateLimit
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return nil, Usage{}, ErrTimeout
	}
	if res.StatusCode == http.StatusUnprocessableEntity && kind == "tts" {
		return nil, Usage{}, ErrInvalidAudio
	}
	if res.StatusCode != http.StatusOK {
		return nil, Usage{}, fmt.Errorf("供应商返回 %d", res.StatusCode)
	}
	var out struct {
		Output     json.RawMessage `json:"output"`
		UsageKnown bool            `json:"usage_known"`
		CostMicros int64           `json:"cost_micros"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, Usage{}, fmt.Errorf("%w: %v", ErrModelJSON, err)
	}
	if len(out.Output) == 0 {
		return nil, Usage{}, ErrInvalidAudio
	}
	return out.Output, Usage{Known: out.UsageKnown, Micros: out.CostMicros}, nil
}

// RequirePublicTLS refuses a public listen without a certificate.
func RequirePublicTLS(public bool, cert, key string) error {
	if !public {
		return nil
	}
	if strings.TrimSpace(cert) == "" || strings.TrimSpace(key) == "" {
		return errors.New("对外监听需要 HTTPS 证书")
	}
	return nil
}

// Routes serves the workspace API. bootstrap issues tokens and is never logged.
func Routes(g *Gateway, bootstrap string, logger *log.Logger) http.Handler {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Qingfan-Bootstrap")
		if bootstrap == "" || subtle.ConstantTimeCompare([]byte(got), []byte(bootstrap)) != 1 {
			writeErr(w, http.StatusForbidden, errors.New("不能发放令牌"))
			logger.Printf("http %s %s %d", r.Method, r.URL.Path, http.StatusForbidden)
			return
		}
		var body struct {
			TenantID string `json:"tenant_id"`
			Quota    int    `json:"quota"`
		}
		if err := readJSON(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		plain, err := g.Issue(body.TenantID, body.Quota)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"token": plain})
		logger.Printf("http %s %s %d", r.Method, r.URL.Path, http.StatusOK)
	})
	mux.HandleFunc("POST /v1/tokens/revoke", func(w http.ResponseWriter, r *http.Request) {
		plain, err := bearer(r)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, err)
			return
		}
		if err := g.Revoke(plain); err != nil {
			writeErr(w, statusOfErr(err), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
		logger.Printf("http %s %s %d", r.Method, r.URL.Path, http.StatusOK)
	})
	mux.HandleFunc("GET /v1/usage", func(w http.ResponseWriter, r *http.Request) {
		plain, err := bearer(r)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, err)
			return
		}
		report, err := g.Costs(plain)
		if err != nil {
			writeErr(w, statusOfErr(err), err)
			return
		}
		writeJSON(w, http.StatusOK, report)
	})
	mux.HandleFunc("POST /v1/scripts", func(w http.ResponseWriter, r *http.Request) {
		plain, err := bearer(r)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, err)
			logger.Printf("http %s %s %d", r.Method, r.URL.Path, http.StatusUnauthorized)
			return
		}
		var body struct {
			TenantID string `json:"tenant_id"`
			TaskID   string `json:"task_id"`
			Facts    []Fact `json:"facts"`
			Clips    []Clip `json:"clips"`
		}
		if err := readJSON(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		row, err := g.lookup(plain)
		if err != nil || row.Revoked {
			status := statusOfErr(err)
			if row.Revoked {
				err = ErrRevoked
				status = http.StatusUnauthorized
			}
			writeErr(w, status, err)
			return
		}
		if body.TenantID != "" && body.TenantID != row.Tenant {
			writeErr(w, http.StatusForbidden, ErrTenant)
			return
		}
		gen, err := Generate(r.Context(), g, plain, body.TaskID, body.Facts, body.Clips)
		if err != nil {
			writeErr(w, statusOfErr(err), err)
			return
		}
		writeJSON(w, http.StatusOK, gen)
		logger.Printf("http %s %s %d", r.Method, r.URL.Path, http.StatusOK)
	})
	return mux
}

func bearer(r *http.Request) (string, error) {
	value, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || strings.TrimSpace(value) == "" || value == r.Header.Get("Authorization") {
		return "", ErrUnauthorized
	}
	return strings.TrimSpace(value), nil
}

func readJSON(r *http.Request, dest any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("%w: %v", ErrModelJSON, err)
	}
	return nil
}

func writeErr(w http.ResponseWriter, code int, err error) {
	if code == 0 {
		code = http.StatusBadRequest
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(body)
}

func statusOfErr(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrRevoked):
		return http.StatusUnauthorized
	case errors.Is(err, ErrTenant):
		return http.StatusForbidden
	case errors.Is(err, ErrBusy), errors.Is(err, ErrRateLimit), errors.Is(err, ErrQuota):
		return http.StatusTooManyRequests
	case errors.Is(err, ErrTimeout):
		return http.StatusRequestTimeout
	default:
		return http.StatusUnprocessableEntity
	}
}
