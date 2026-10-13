package ai

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
)

const (
	ErrUnauthorized = gatewayError("未授权")
	ErrRevoked      = gatewayError("令牌已撤销")
	ErrTenant       = gatewayError("不能访问其他工作空间")
	ErrBusy         = gatewayError("同时调用已达上限")
	ErrQuota        = gatewayError("调用额度已用完")
	ErrRetry        = gatewayError("超过重试次数")
	ErrTimeout      = gatewayError("调用超时")
	ErrRateLimit    = gatewayError("供应商限流")
	ErrInvalidAudio = gatewayError("配音音频无效")
)

type gatewayError string

func (e gatewayError) Error() string { return string(e) }

// Call is one supplier attempt. Tenant is checked against the token and ignored when empty.
type Call struct {
	Tenant        string
	TaskID        string
	SegmentID     string
	Kind          string
	Model         string
	PromptVersion string
	Voice         string
	Input         []byte
	Attempt       int
}

// Result is a supplier body. Cached results were not billed again.
type Result struct {
	Body       []byte
	Cached     bool
	Status     string
	CostKnown  bool
	CostMicros int64
}

// Provider is one vision/text supplier and one speech supplier behind the same gateway.
type Provider interface {
	Name() string
	Complete(ctx context.Context, kind string, input []byte) (body []byte, usage Usage, err error)
}

// Gateway checks tokens, concurrency, quota, and writes an append-only usage row per attempt.
type Gateway struct {
	db       *sql.DB
	provider Provider
	limit    int
	mu       sync.Mutex
	inflight int
	logger   *log.Logger
}

func Open(path string, provider Provider, limit int) (*Gateway, error) {
	if provider == nil {
		return nil, errors.New("没有配置供应商")
	}
	if limit < 1 {
		limit = 2
	}
	db, err := openUsage(path)
	if err != nil {
		return nil, err
	}
	return &Gateway{db: db, provider: provider, limit: limit, logger: log.New(io.Discard, "", 0)}, nil
}

func (g *Gateway) SetLogger(logger *log.Logger) {
	if logger != nil {
		g.logger = logger
	}
}

func (g *Gateway) Close() error { return g.db.Close() }

func (g *Gateway) acquire() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.inflight >= g.limit {
		return ErrBusy
	}
	g.inflight++
	return nil
}

func (g *Gateway) release() {
	g.mu.Lock()
	g.inflight--
	g.mu.Unlock()
}

// Do performs one attempt. Attempt must be from 1 to MaxAttempts.
// The same input for the same tenant is served from cache and is not billed again.
func (g *Gateway) Do(ctx context.Context, plain string, call Call) (Result, error) {
	if plain == "" {
		return Result{}, ErrUnauthorized
	}
	if call.Attempt < 1 || call.Attempt > MaxAttempts {
		return Result{}, ErrRetry
	}
	if call.Kind == "" || call.TaskID == "" || call.Model == "" || call.PromptVersion == "" {
		return Result{}, fmt.Errorf("调用缺少类型、任务、模型或提示词版本")
	}
	if err := g.acquire(); err != nil {
		return Result{}, err
	}
	defer g.release()
	row, err := g.lookup(plain)
	if err != nil {
		return Result{}, err
	}
	if row.Revoked {
		return Result{}, ErrRevoked
	}
	if call.Tenant != "" && call.Tenant != row.Tenant {
		return Result{}, ErrTenant
	}
	call.Tenant = row.Tenant
	sum := hashInput(call.Kind, call.Voice, call.Model, call.PromptVersion, call.Input)
	if body, ok, err := g.cached(row.Tenant, call.Kind, call.Voice, sum); err != nil {
		return Result{}, err
	} else if ok {
		if err := g.recordCache(row, call, sum, g.provider.Name()); err != nil {
			return Result{}, err
		}
		g.log(call, "cache")
		return Result{Body: body, Cached: true, Status: "cache"}, nil
	}
	id, err := g.reserve(row, call, sum, g.provider.Name())
	if err != nil {
		return Result{}, err
	}
	body, usage, callErr := g.provider.Complete(ctx, call.Kind, call.Input)
	if callErr == nil && len(body) == 0 {
		callErr = ErrInvalidAudio
	}
	status := statusOf(ctx, callErr)
	if err := g.finish(id, status, usage, callErr == nil); err != nil {
		return Result{}, err
	}
	g.log(call, status)
	if callErr != nil {
		return Result{}, callErr
	}
	if len(body) == 0 {
		return Result{}, ErrInvalidAudio
	}
	if err := g.remember(row.Tenant, call.Kind, call.Voice, sum, string(body)); err != nil {
		return Result{}, err
	}
	return Result{Body: body, Status: status, CostKnown: usage.Known, CostMicros: usage.Micros}, nil
}

func (g *Gateway) log(call Call, status string) {
	g.logger.Printf("ai kind=%s status=%s task=%s", call.Kind, status, call.TaskID)
}

func statusOf(ctx context.Context, err error) string {
	switch {
	case err == nil:
		return "succeeded"
	case errors.Is(err, ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, ErrRateLimit):
		return "rate_limited"
	case errors.Is(err, ErrInvalidAudio):
		return "invalid_audio"
	default:
		if ctx.Err() != nil {
			return "timeout"
		}
		return "failed"
	}
}
