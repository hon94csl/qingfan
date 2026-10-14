package main

import (
	"github.com/hon94csl/qingfan/internal/ai"
	"github.com/hon94csl/qingfan/internal/reporting"
)

// Usage reads the workspace ledger. Amounts stay in known micros and unknown calls.
func (a *App) Usage() (ai.CostReport, error) {
	plain, err := a.token()
	if err != nil {
		return ai.CostReport{}, err
	}
	return a.ai.Costs(plain)
}

// BindPublication records one post against an existing export.
func (a *App) BindPublication(outputID, url, videoID, shop, publishedAt string) (reporting.Publication, error) {
	if err := a.ready(); err != nil {
		return reporting.Publication{}, err
	}
	return a.db.BindPublication(outputID, url, videoID, shop, publishedAt)
}

// PreviewMetrics checks a CSV without importing it.
func (a *App) PreviewMetrics(csv string) (reporting.Preview, error) {
	if err := a.ready(); err != nil {
		return reporting.Preview{}, err
	}
	return a.db.PreviewMetrics([]byte(csv))
}

// ImportMetrics stores new video-day rows from the agreed CSV template.
func (a *App) ImportMetrics(csv string) (reporting.Preview, error) {
	if err := a.ready(); err != nil {
		return reporting.Preview{}, err
	}
	return a.db.ImportMetrics([]byte(csv))
}
