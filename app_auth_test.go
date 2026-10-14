package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hon94csl/qingfan/internal/auth"
)

func TestDesktopLoginGatesTheWorkbench(t *testing.T) {
	t.Setenv("QINGFAN_DATA", t.TempDir())
	app := &App{}
	app.startup(context.Background())
	t.Cleanup(func() { app.shutdown(context.Background()) })
	if app.authErr != nil {
		t.Fatal(app.authErr)
	}

	if _, err := app.CreateProject("试用", "ID", "id-ID"); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("work before login: %v", err)
	}
	if tools := app.CheckMediaTools(); tools != nil {
		t.Fatalf("tools before login: %+v", tools)
	}
	if current, err := app.CurrentAccount(); err != nil || current.Email != "" {
		t.Fatalf("current before login: %+v %v", current, err)
	}

	if _, err := app.auth.CreateDesktop("Desk@Example.com", "long-test-password"); err != nil {
		t.Fatal(err)
	}
	if app.db != nil {
		app.db.Close()
	}
	app.db = nil
	app.storeErr = fmt.Errorf("素材库还没有打开")
	signedIn, err := app.Login("desk@example.com", "long-test-password")
	if err != nil || signedIn.Email != "desk@example.com" || signedIn.Kind != auth.KindDesktop {
		t.Fatalf("desktop login: %+v %v", signedIn, err)
	}
	if _, err := app.CreateProject("试用", "ID", "id-ID"); err == nil || errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("login should reach the closed library: %v", err)
	}

	admin, err := app.auth.Create("ops@example.com", "another-long-password")
	if err != nil {
		t.Fatal(err)
	}
	signedIn, err = app.Login(admin.Email, "another-long-password")
	if err != nil || signedIn.Kind != auth.KindAdmin {
		t.Fatalf("admin may try the desktop: %+v %v", signedIn, err)
	}
	if err := app.Logout(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ListClips("sku"); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("after logout: %v", err)
	}
	if current, err := app.CurrentAccount(); err != nil || current.Email != "" {
		t.Fatalf("current after logout: %+v %v", current, err)
	}
}

func TestRegisterFollowsTheAdminSwitch(t *testing.T) {
	t.Setenv("QINGFAN_DATA", t.TempDir())
	app := &App{}
	app.startup(context.Background())
	t.Cleanup(func() { app.shutdown(context.Background()) })
	if app.authErr != nil {
		t.Fatal(app.authErr)
	}
	open, err := app.RegistrationOpen()
	if err != nil || open {
		t.Fatalf("default: %v %v", open, err)
	}
	if _, err := app.Register("new@example.com", "long-test-password"); !errors.Is(err, auth.ErrRegistrationClosed) {
		t.Fatalf("closed: %v", err)
	}
	if err := app.auth.SetRegistrationOpen(true); err != nil {
		t.Fatal(err)
	}
	account, err := app.Register("New@Example.com", "long-test-password")
	if err != nil || account.Email != "new@example.com" || account.Kind != auth.KindDesktop {
		t.Fatalf("register: %+v %v", account, err)
	}
	if current, err := app.CurrentAccount(); err != nil || current.Email != account.Email {
		t.Fatalf("session: %+v %v", current, err)
	}
	if n, err := app.auth.Count(); err != nil || n != 0 {
		t.Fatalf("a desktop registration must not create an administrator: %d %v", n, err)
	}
	if err := app.auth.SetRegistrationOpen(false); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Register("other@example.com", "another-long-password"); !errors.Is(err, auth.ErrRegistrationClosed) {
		t.Fatalf("closed again: %v", err)
	}
	if _, err := app.Login("new@example.com", "long-test-password"); err != nil {
		t.Fatalf("existing account: %v", err)
	}
}
