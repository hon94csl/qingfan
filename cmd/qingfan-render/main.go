package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hon94csl/qingfan/internal/media"
	"os"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "doctor" {
		fmt.Fprintln(os.Stderr, "usage: qingfan-render doctor (rendering arrives in M1)")
		os.Exit(2)
	}
	statuses := media.CheckTools(context.Background())
	if err := json.NewEncoder(os.Stdout).Encode(statuses); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, status := range statuses {
		if status.Error != "" {
			os.Exit(1)
		}
	}
}
