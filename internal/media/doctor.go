package media

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

type ToolStatus struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
	Error   string `json:"error,omitempty"`
}

func CheckTools(ctx context.Context) []ToolStatus {
	return []ToolStatus{checkTool(ctx, "ffmpeg"), checkTool(ctx, "ffprobe")}
}

func checkTool(ctx context.Context, name string) ToolStatus {
	status := ToolStatus{Name: name}
	path, err := exec.LookPath(name)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Path = path
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "-version").Output()
	if err != nil {
		status.Error = err.Error()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			status.Error += ": " + strings.TrimSpace(string(exitErr.Stderr))
		}
		return status
	}
	status.Version = strings.SplitN(strings.TrimSpace(string(output)), "\n", 2)[0]
	if status.Version == "" {
		status.Error = "empty version output"
	}
	return status
}
