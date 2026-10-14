package store

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hon94csl/qingfan/internal/auth"
)

func TestCreateShopStaysOnTheAccount(t *testing.T) {
	dir := t.TempDir()
	s, err := open(filepath.Join(dir, "qingfan.sqlite"), filepath.Join(dir, "cache"), (&fakeIngest{}).Ingest)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	if _, err := s.CreateShop("", "店"); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("missing account: %v", err)
	}
	if _, err := s.CreateShop("acct-a", "  "); !errors.Is(err, ErrShopName) {
		t.Fatalf("blank name: %v", err)
	}
	if _, err := s.CreateShop("acct-a", strings.Repeat("店", 81)); !errors.Is(err, ErrShopLong) {
		t.Fatalf("long name: %v", err)
	}
	first, err := s.CreateShop("acct-a", " 一店 ")
	if err != nil || first.Name != "一店" || first.ID == "" || first.CreatedAt == "" {
		t.Fatalf("create %+v %v", first, err)
	}
	if _, err := s.CreateShop("acct-a", "二店"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateShop("acct-a", "一店"); !errors.Is(err, ErrShopExists) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := s.CreateShop("acct-b", "一店"); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListShops("acct-a")
	if err != nil || len(rows) != 2 || rows[0].Name != "一店" || rows[1].Name != "二店" {
		t.Fatalf("rows %+v %v", rows, err)
	}
	other, err := s.ListShops("acct-b")
	if err != nil || len(other) != 1 || other[0].Name != "一店" || other[0].ID == first.ID {
		t.Fatalf("other %+v %v", other, err)
	}
	none, err := s.ListShops("")
	if err != nil || len(none) != 0 {
		t.Fatalf("empty account %+v %v", none, err)
	}
}
