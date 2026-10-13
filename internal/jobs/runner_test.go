package jobs

import (
	"context"
	"errors"
	"testing"
)

type book struct {
	id     string
	status string
	msg    string
}

func (b *book) StartJob(kind string) (string, error) {
	b.id = kind + "-1"
	b.status = "running"
	return b.id, nil
}

func (b *book) FinishJob(id, status, message string) error {
	if id != b.id {
		return errors.New("wrong id")
	}
	b.status = status
	b.msg = message
	return nil
}

func TestRunRecordsFailure(t *testing.T) {
	b := &book{}
	err := Run(context.Background(), b, "import", func(context.Context) error {
		return errors.New("探测失败")
	})
	if err == nil || b.status != "failed" || b.msg != "探测失败" {
		t.Fatalf("err=%v status=%s msg=%s", err, b.status, b.msg)
	}
}

func TestRunRecordsSuccess(t *testing.T) {
	b := &book{}
	if err := Run(context.Background(), b, "export", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if b.status != "succeeded" || b.msg != "" {
		t.Fatalf("%s %s", b.status, b.msg)
	}
}
