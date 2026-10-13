package jobs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	Pending     = "pending"
	Running     = "running"
	Succeeded   = "succeeded"
	Failed      = "failed"
	Cancelled   = "cancelled"
	NeedsReview = "needs_review"
)

// Step is one recoverable unit of work.
// Progress is recorded for display and is never evidence that the step succeeded.
type Step struct {
	ID       string
	Key      string
	Status   string
	Artifact string
	Progress int
}

// FileReady reports whether path is a non-empty regular file.
func FileReady(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

// Recover restarts work that was interrupted or whose artifact is missing.
// Failed, cancelled, and needs_review steps are left as they are.
// The input slice is not modified.
func Recover(steps []Step, ready func(string) bool) []Step {
	if ready == nil {
		ready = func(string) bool { return false }
	}
	out := make([]Step, len(steps))
	copy(out, steps)
	for i := range out {
		switch out[i].Status {
		case Running:
			out[i].Status = Pending
			out[i].Artifact = ""
		case Succeeded:
			if !ready(out[i].Artifact) {
				out[i].Status = Pending
				out[i].Artifact = ""
			}
		}
	}
	return out
}

// Cleanup removes cache files whose names are not in keep.
// Paths outside dir are refused. Symlinks are removed without deleting their targets.
// Protected paths, including originals and successful exports, are not removed.
func Cleanup(dir string, keep map[string]bool, protected map[string]bool) ([]string, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	safe := map[string]bool{}
	for path := range protected {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		safe[abs] = true
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			safe[resolved] = true
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, entry := range entries {
		name := entry.Name()
		if keep[name] {
			continue
		}
		path := filepath.Join(dir, name)
		abs, err := filepath.Abs(path)
		if err != nil {
			return removed, err
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return removed, fmt.Errorf("拒绝清理缓存目录之外的路径")
		}
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		if safe[abs] {
			continue
		}
		if err := os.Remove(path); err != nil {
			return removed, err
		}
		removed = append(removed, abs)
	}
	return removed, nil
}
