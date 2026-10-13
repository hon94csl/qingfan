package credentials

import (
	"fmt"
	"os/exec"
	"runtime"
	"sync"
)

const service = "qingfan-workspace"

// Store keeps the workspace token outside the frontend bundle.
type Store interface {
	Put(account, secret string) error
	Get(account string) (string, error)
}

// Memory is an in-process store for tests and non-macOS sessions.
type Memory struct {
	mu    sync.Mutex
	items map[string]string
}

func (m *Memory) Put(account, secret string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.items == nil {
		m.items = map[string]string{}
	}
	m.items[account] = secret
	return nil
}

func (m *Memory) Get(account string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	secret, ok := m.items[account]
	if !ok || secret == "" {
		return "", fmt.Errorf("系统凭据库里没有工作空间令牌")
	}
	return secret, nil
}

// Runner executes one command. Tests pass a fake so the real keychain is untouched.
type Runner func(name string, args ...string) ([]byte, error)

// Keychain stores the token with macOS `security`. Errors never include the token.
func Keychain(run Runner) Store {
	if run == nil {
		run = func(name string, args ...string) ([]byte, error) {
			return exec.Command(name, args...).CombinedOutput()
		}
	}
	return keychain{run: run}
}

type keychain struct{ run Runner }

func (k keychain) Put(account, secret string) error {
	_, err := k.run("security", "add-generic-password", "-U", "-s", service, "-a", account, "-w", secret)
	if err != nil {
		return fmt.Errorf("无法写入系统凭据库")
	}
	return nil
}

func (k keychain) Get(account string) (string, error) {
	out, err := k.run("security", "find-generic-password", "-s", service, "-a", account, "-w")
	if err != nil {
		return "", fmt.Errorf("系统凭据库里没有工作空间令牌")
	}
	if len(out) == 0 {
		return "", fmt.Errorf("系统凭据库里没有工作空间令牌")
	}
	return string(trimLine(out)), nil
}

func trimLine(raw []byte) []byte {
	for len(raw) > 0 && (raw[len(raw)-1] == '\n' || raw[len(raw)-1] == '\r') {
		raw = raw[:len(raw)-1]
	}
	return raw
}

// Default uses the macOS keychain. Other systems keep the token in memory for this process.
func Default() Store {
	if runtime.GOOS == "darwin" {
		return Keychain(nil)
	}
	return &Memory{}
}
