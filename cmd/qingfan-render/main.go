package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/hon94csl/qingfan/internal/media"
	"github.com/hon94csl/qingfan/internal/timeline"
)

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	if len(args) < 2 {
		usage()
		return 2
	}
	switch args[1] {
	case "doctor":
		return doctor()
	case "render":
		return render(args[2:])
	default:
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: qingfan-render doctor")
	fmt.Fprintln(os.Stderr, "       qingfan-render render -timeline FILE -assets FILE -output FILE [-timeout 10m]")
}

func doctor() int {
	statuses := media.CheckTools(context.Background())
	if err := json.NewEncoder(os.Stdout).Encode(statuses); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, status := range statuses {
		if status.Error != "" {
			return 1
		}
	}
	return 0
}

func render(args []string) int {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	timelinePath := fs.String("timeline", "", "render timeline json")
	assetsPath := fs.String("assets", "", "asset catalog json")
	outputPath := fs.String("output", "", "output mp4 path")
	timeout := fs.Duration("timeout", 10*time.Minute, "render timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *timelinePath == "" || *assetsPath == "" || *outputPath == "" {
		fs.Usage()
		return 2
	}
	tl, err := timeline.LoadTimeline(*timelinePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	assets, err := timeline.LoadAssets(*assetsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	assets = timeline.ResolvePaths(assets, filepath.Dir(*assetsPath))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	manifest, err := media.Render(ctx, tl, assets, *outputPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(manifest); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
