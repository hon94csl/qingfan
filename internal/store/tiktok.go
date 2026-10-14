package store

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/hon94csl/qingfan/internal/auth"
	"github.com/hon94csl/qingfan/internal/tiktok"
)

// TikTokShop is a connected shop the browser may see. Tokens and the shop cipher stay in the database.
type TikTokShop struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Region     string `json:"region"`
	SellerType string `json:"seller_type"`
	Code       string `json:"code"`
	SellerName string `json:"seller_name"`
	UpdatedAt  string `json:"updated_at"`
}

// TikTokVideo is one official video performance snapshot for a requested date range.
type TikTokVideo struct {
	ShopID   string `json:"shop_id"`
	ShopName string `json:"shop_name"`
	VideoID  string `json:"video_id"`
	Title    string `json:"title"`
	Account  string `json:"account"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Views    *int   `json:"views"`
	Orders   *int   `json:"orders"`
	GMVMinor *int   `json:"gmv_minor"`
	Currency string `json:"currency"`
	Latest   string `json:"latest_available"`
}

// TikTokSecret is the server-only credential for one shop.
type TikTokSecret struct {
	ShopID        string
	Cipher        string
	AccessToken   string
	RefreshToken  string
	AccessExpire  int64
	RefreshExpire int64
	OpenID        string
}

// SaveTikTokState stores a single-use seller authorization state for the account that started it.
func (s *Store) SaveTikTokState(state, accountID string, expires int64) error {
	if accountID == "" {
		return auth.ErrUnauthorized
	}
	_, err := s.db.Exec(`INSERT INTO tiktok_oauth_states(state, account_id, expires_at) VALUES (?, ?, ?)`, state, accountID, expires)
	return err
}

// ConsumeTikTokState accepts a state once and returns the account that started it.
// A missing, expired, or ownerless state is rejected.
func (s *Store) ConsumeTikTokState(state string, now int64) (string, error) {
	var accountID string
	var expires int64
	err := s.db.QueryRow(`SELECT account_id, expires_at FROM tiktok_oauth_states WHERE state=?`, state).Scan(&accountID, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return "", tiktok.ErrState
	}
	if err != nil {
		return "", err
	}
	if _, err := s.db.Exec(`DELETE FROM tiktok_oauth_states WHERE state=?`, state); err != nil {
		return "", err
	}
	if expires <= now || accountID == "" {
		return "", tiktok.ErrState
	}
	return accountID, nil
}

// SaveTikTokAuth writes the seller token onto each authorized shop for one account.
// Shops already stored for that account are updated. A shop owned by someone else is rejected,
// and earlier shops from the same call are not kept.
func (s *Store) SaveTikTokAuth(accountID string, token tiktok.Token, shops []tiktok.Shop) error {
	if accountID == "" {
		return auth.ErrUnauthorized
	}
	if len(shops) == 0 {
		return tiktok.ErrNoShop
	}
	scopes, err := json.Marshal(token.Scopes)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	updated := nowText()
	for _, shop := range shops {
		if shop.ID == "" || shop.Cipher == "" {
			tx.Rollback()
			return tiktok.ErrNoShop
		}
		var owner string
		err := tx.QueryRow(`SELECT account_id FROM tiktok_shops WHERE id=?`, shop.ID).Scan(&owner)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			tx.Rollback()
			return err
		}
		if err == nil && owner != "" && owner != accountID {
			tx.Rollback()
			return tiktok.ErrOwned
		}
		_, err = tx.Exec(`INSERT INTO tiktok_shops(
			id, account_id, name, region, seller_type, cipher, code, open_id, seller_name,
			access_token, refresh_token, access_expire, refresh_expire, scopes, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			account_id=excluded.account_id,
			name=excluded.name, region=excluded.region, seller_type=excluded.seller_type,
			cipher=excluded.cipher, code=excluded.code, open_id=excluded.open_id, seller_name=excluded.seller_name,
			access_token=excluded.access_token, refresh_token=excluded.refresh_token,
			access_expire=excluded.access_expire, refresh_expire=excluded.refresh_expire,
			scopes=excluded.scopes, updated_at=excluded.updated_at`,
			shop.ID, accountID, shop.Name, shop.Region, shop.SellerType, shop.Cipher, shop.Code, token.OpenID, token.SellerName,
			token.AccessToken, token.RefreshToken, token.AccessExpire, token.RefreshExpire, string(scopes), updated)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ListTikTokShops returns one account's shops without tokens or ciphers.
// An empty account returns no rows.
func (s *Store) ListTikTokShops(accountID string) ([]TikTokShop, error) {
	if accountID == "" {
		return []TikTokShop{}, nil
	}
	rows, err := s.db.Query(`SELECT id, name, region, seller_type, code, seller_name, updated_at FROM tiktok_shops WHERE account_id=? ORDER BY name, id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	shops := []TikTokShop{}
	for rows.Next() {
		var shop TikTokShop
		if err := rows.Scan(&shop.ID, &shop.Name, &shop.Region, &shop.SellerType, &shop.Code, &shop.SellerName, &shop.UpdatedAt); err != nil {
			return nil, err
		}
		shops = append(shops, shop)
	}
	return shops, rows.Err()
}

// TikTokSecret loads the server-only credential for one shop of one account.
func (s *Store) TikTokSecret(accountID, shopID string) (TikTokSecret, error) {
	if accountID == "" || shopID == "" {
		return TikTokSecret{}, tiktok.ErrShop
	}
	var secret TikTokSecret
	err := s.db.QueryRow(`SELECT id, cipher, access_token, refresh_token, access_expire, refresh_expire, open_id FROM tiktok_shops WHERE id=? AND account_id=?`, shopID, accountID).Scan(
		&secret.ShopID, &secret.Cipher, &secret.AccessToken, &secret.RefreshToken, &secret.AccessExpire, &secret.RefreshExpire, &secret.OpenID)
	if errors.Is(err, sql.ErrNoRows) {
		return TikTokSecret{}, tiktok.ErrShop
	}
	return secret, err
}

// UpdateTikTokToken replaces the access grant on one account's shops for one seller.
func (s *Store) UpdateTikTokToken(accountID, openID string, token tiktok.Token) error {
	if accountID == "" || openID == "" {
		return tiktok.ErrShop
	}
	scopes, err := json.Marshal(token.Scopes)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`UPDATE tiktok_shops SET access_token=?, refresh_token=?, access_expire=?, refresh_expire=?, scopes=?, seller_name=?, updated_at=? WHERE open_id=? AND account_id=?`,
		token.AccessToken, token.RefreshToken, token.AccessExpire, token.RefreshExpire, string(scopes), token.SellerName, nowText(), openID, accountID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return tiktok.ErrShop
	}
	return nil
}

// ReplaceShopVideos replaces one shop's snapshot for one inclusive-start, exclusive-end range.
// It does not write metrics_daily, and it leaves other ranges in place.
func (s *Store) ReplaceShopVideos(accountID, shopID, start, end, latest string, videos []tiktok.Video) error {
	if accountID == "" {
		return tiktok.ErrShop
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM tiktok_shops WHERE id=? AND account_id=?`, shopID, accountID).Scan(&n); err != nil {
		tx.Rollback()
		return err
	}
	if n == 0 {
		tx.Rollback()
		return tiktok.ErrShop
	}
	if _, err := tx.Exec(`DELETE FROM tiktok_video_facts WHERE shop_id=? AND start_date=? AND end_date=?`, shopID, start, end); err != nil {
		tx.Rollback()
		return err
	}
	fetched := nowText()
	seen := map[string]tiktok.Video{}
	order := make([]string, 0, len(videos))
	for _, video := range videos {
		if video.ID == "" {
			continue
		}
		if _, ok := seen[video.ID]; !ok {
			order = append(order, video.ID)
		}
		seen[video.ID] = video
	}
	for _, id := range order {
		video := seen[id]
		_, err := tx.Exec(`INSERT INTO tiktok_video_facts(
			shop_id, video_id, title, account, start_date, end_date, views, orders, gmv_minor, currency, latest_available, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			shopID, video.ID, video.Title, video.Username, start, end,
			metricValue(video.Views), metricValue(video.Orders), metricValue(video.GMVMinor),
			video.Currency, latest, fetched)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ListTikTokVideos returns one account's snapshots. An empty shop ID returns every shop of that account.
// An empty account returns no rows.
func (s *Store) ListTikTokVideos(accountID, shopID string) ([]TikTokVideo, error) {
	if accountID == "" {
		return []TikTokVideo{}, nil
	}
	rows, err := s.db.Query(`SELECT f.shop_id, s.name, f.video_id, f.title, f.account, f.start_date, f.end_date,
		f.views, f.orders, f.gmv_minor, f.currency, f.latest_available
		FROM tiktok_video_facts f JOIN tiktok_shops s ON s.id = f.shop_id
		WHERE s.account_id = ? AND (? = '' OR f.shop_id = ?)
		ORDER BY f.start_date, f.video_id`, accountID, shopID, shopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	videos := []TikTokVideo{}
	for rows.Next() {
		var video TikTokVideo
		var views, orders, gmv sql.NullInt64
		if err := rows.Scan(&video.ShopID, &video.ShopName, &video.VideoID, &video.Title, &video.Account, &video.Start, &video.End,
			&views, &orders, &gmv, &video.Currency, &video.Latest); err != nil {
			return nil, err
		}
		video.Views = nullInt(views)
		video.Orders = nullInt(orders)
		video.GMVMinor = nullInt(gmv)
		videos = append(videos, video)
	}
	return videos, rows.Err()
}

func nullInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	n := int(value.Int64)
	return &n
}
