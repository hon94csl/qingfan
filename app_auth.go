package main

import (
	"fmt"

	"github.com/hon94csl/qingfan/internal/auth"
)

func (a *App) accounts() (*auth.Store, error) {
	if a.authErr != nil {
		return nil, a.authErr
	}
	if a.auth == nil {
		return nil, fmt.Errorf("账号库还没有打开")
	}
	return a.auth, nil
}

func (a *App) requireAccount() (auth.Account, error) {
	a.authMu.Lock()
	defer a.authMu.Unlock()
	if a.who.Email == "" {
		return auth.Account{}, auth.ErrUnauthorized
	}
	return a.who, nil
}

// Login checks a desktop or administrator password and keeps the session in this process.
// The session token stays here so the page cannot read it.
func (a *App) Login(email, password string) (auth.Account, error) {
	store, err := a.accounts()
	if err != nil {
		return auth.Account{}, err
	}
	account, token, err := store.Login(email, password)
	if err != nil {
		return auth.Account{}, err
	}
	return a.beginSession(store, account, token), nil
}

// RegistrationOpen reports the switch set in the admin console.
func (a *App) RegistrationOpen() (bool, error) {
	store, err := a.accounts()
	if err != nil {
		return false, err
	}
	return store.RegistrationOpen()
}

// Register creates a desktop account and signs it in when the admin console has opened registration.
func (a *App) Register(email, password string) (auth.Account, error) {
	store, err := a.accounts()
	if err != nil {
		return auth.Account{}, err
	}
	open, err := store.RegistrationOpen()
	if err != nil {
		return auth.Account{}, err
	}
	if !open {
		return auth.Account{}, auth.ErrRegistrationClosed
	}
	if _, err := store.CreateDesktop(email, password); err != nil {
		return auth.Account{}, err
	}
	account, token, err := store.Login(email, password)
	if err != nil {
		return auth.Account{}, err
	}
	return a.beginSession(store, account, token), nil
}

func (a *App) beginSession(store *auth.Store, account auth.Account, token string) auth.Account {
	a.authMu.Lock()
	previous := a.session
	a.session = token
	a.who = account
	a.authMu.Unlock()
	if previous != "" && previous != token {
		_ = store.Logout(previous)
	}
	return account
}

// CurrentAccount returns the signed-in person. An empty email means nobody is signed in.
func (a *App) CurrentAccount() (auth.Account, error) {
	a.authMu.Lock()
	defer a.authMu.Unlock()
	return a.who, nil
}

// Logout ends the desktop session. Opening the app again requires a password.
func (a *App) Logout() error {
	a.authMu.Lock()
	token := a.session
	a.session = ""
	a.who = auth.Account{}
	a.authMu.Unlock()
	if a.auth == nil {
		return nil
	}
	return a.auth.Logout(token)
}
