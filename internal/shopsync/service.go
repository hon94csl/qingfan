// Package shopsync starts a TikTok Shop seller authorization and writes video performance into the business database.
package shopsync

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/hon94csl/qingfan/internal/auth"
	"github.com/hon94csl/qingfan/internal/store"
	"github.com/hon94csl/qingfan/internal/tiktok"
)

const stateTTL = 30 * time.Minute

// Service uses the official client and the shared business database.
type Service struct {
	DB        *store.Store
	Client    *tiktok.Client
	Callback  string
	PageLimit int
}

// ConnectResult is the seller link and the state the callback must echo.
type ConnectResult struct {
	URL   string `json:"url"`
	State string `json:"state"`
}

// SyncResult is what was written for one date range.
type SyncResult struct {
	Written int    `json:"written"`
	Note    string `json:"note"`
	Latest  string `json:"latest_available"`
}

// Status tells the signed-in desktop whether the app credentials exist. It includes no secrets.
type Status struct {
	Configured bool   `json:"configured"`
	Callback   string `json:"callback"`
}

// Status reports the app configuration and the callback to register in Partner Center.
func (s *Service) Status() Status {
	configured := s != nil && s.Client != nil && s.Client.Configured()
	callback := ""
	if s != nil {
		callback = s.Callback
	}
	return Status{Configured: configured, Callback: callback}
}

// Connect starts a seller authorization for one account. The state expires in 30 minutes and can be used once.
func (s *Service) Connect(ctx context.Context, accountID string) (ConnectResult, error) {
	client, err := s.ready()
	if err != nil {
		return ConnectResult{}, err
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return ConnectResult{}, auth.ErrUnauthorized
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return ConnectResult{}, err
	}
	state := hex.EncodeToString(raw[:])
	expires := client.Moment().Add(stateTTL).Unix()
	if err := s.DB.SaveTikTokState(state, accountID, expires); err != nil {
		return ConnectResult{}, err
	}
	link, err := client.AuthorizeURL(state)
	if err != nil {
		return ConnectResult{}, err
	}
	return ConnectResult{URL: link, State: state}, nil
}

// Complete exchanges an authorization code and writes every returned shop onto the account that started the link.
func (s *Service) Complete(ctx context.Context, code, state string) ([]store.TikTokShop, error) {
	client, err := s.ready()
	if err != nil {
		return nil, err
	}
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	if code == "" {
		return nil, tiktok.ErrCode
	}
	accountID, err := s.DB.ConsumeTikTokState(state, client.Moment().Unix())
	if err != nil {
		return nil, err
	}
	token, err := client.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	shops, err := client.Shops(ctx, token.AccessToken)
	if err != nil {
		return nil, err
	}
	if err := s.DB.SaveTikTokAuth(accountID, token, shops); err != nil {
		return nil, err
	}
	return s.DB.ListTikTokShops(accountID)
}

// Sync reads one account's shop video performance and replaces that range in the business database.
func (s *Service) Sync(ctx context.Context, accountID, shopID, start, end string) (SyncResult, error) {
	client, err := s.ready()
	if err != nil {
		return SyncResult{}, err
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return SyncResult{}, auth.ErrUnauthorized
	}
	shopID = strings.TrimSpace(shopID)
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)
	if err := validRange(start, end); err != nil {
		return SyncResult{}, err
	}
	secret, err := s.DB.TikTokSecret(accountID, shopID)
	if err != nil {
		return SyncResult{}, err
	}
	access := secret.AccessToken
	if secret.AccessExpire-client.Moment().Unix() < 300 {
		refreshed, err := client.Refresh(ctx, secret.RefreshToken)
		if err != nil {
			return SyncResult{}, err
		}
		if err := s.DB.UpdateTikTokToken(accountID, secret.OpenID, refreshed); err != nil {
			return SyncResult{}, err
		}
		access = refreshed.AccessToken
	}
	limit := s.PageLimit
	if limit <= 0 {
		limit = 50
	}
	var videos []tiktok.Video
	pageToken := ""
	latest := ""
	for page := 0; page < limit; page++ {
		got, err := client.Videos(ctx, access, secret.Cipher, start, end, pageToken)
		if err != nil {
			return SyncResult{}, err
		}
		if got.Latest != "" {
			latest = got.Latest
		}
		videos = append(videos, got.Videos...)
		if got.Next == "" {
			if err := s.DB.ReplaceShopVideos(accountID, shopID, start, end, latest, videos); err != nil {
				return SyncResult{}, err
			}
			written := countVideos(videos)
			note := ""
			if written == 0 {
				note = "这个区间没有视频"
			}
			return SyncResult{Written: written, Note: note, Latest: latest}, nil
		}
		pageToken = got.Next
	}
	return SyncResult{}, tiktok.ErrPartial
}

func (s *Service) ready() (*tiktok.Client, error) {
	if s == nil || s.Client == nil || !s.Client.Configured() {
		return nil, tiktok.ErrConfig
	}
	if s.DB == nil {
		return nil, errors.New("后台数据库还没有打开")
	}
	return s.Client, nil
}

func countVideos(videos []tiktok.Video) int {
	seen := map[string]bool{}
	n := 0
	for _, video := range videos {
		if video.ID == "" || seen[video.ID] {
			continue
		}
		seen[video.ID] = true
		n++
	}
	return n
}

func validRange(start, end string) error {
	if _, err := time.Parse("2006-01-02", start); err != nil {
		return tiktok.ErrRange
	}
	if _, err := time.Parse("2006-01-02", end); err != nil {
		return tiktok.ErrRange
	}
	if start >= end {
		return tiktok.ErrRange
	}
	return nil
}
