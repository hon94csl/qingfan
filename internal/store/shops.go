package store

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/hon94csl/qingfan/internal/auth"
)

const shopNameLimit = 80

var (
	ErrShopName   = errors.New("先填写店铺")
	ErrShopLong   = errors.New("店铺名称过长")
	ErrShopExists = errors.New("这家店铺已经创建")
)

// Shop is a shop created inside Qingfan. It is not a TikTok authorization.
type Shop struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// CreateShop stores one shop on the signed-in account. The same account cannot reuse the name.
func (s *Store) CreateShop(accountID, name string) (Shop, error) {
	if strings.TrimSpace(accountID) == "" {
		return Shop{}, auth.ErrUnauthorized
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Shop{}, ErrShopName
	}
	if utf8.RuneCountInString(name) > shopNameLimit {
		return Shop{}, ErrShopLong
	}
	id, err := newID()
	if err != nil {
		return Shop{}, err
	}
	created := nowText()
	_, err = s.db.Exec(`INSERT INTO shops(id, account_id, name, created_at) VALUES (?, ?, ?, ?)`, id, accountID, name, created)
	if err != nil {
		if strings.Contains(err.Error(), "shops_account_name") || strings.Contains(err.Error(), "UNIQUE constraint failed: shops.account_id") {
			return Shop{}, ErrShopExists
		}
		return Shop{}, err
	}
	return Shop{ID: id, Name: name, CreatedAt: created}, nil
}

// ListShops returns one account's shops. An empty account returns no rows.
func (s *Store) ListShops(accountID string) ([]Shop, error) {
	if strings.TrimSpace(accountID) == "" {
		return []Shop{}, nil
	}
	rows, err := s.db.Query(`SELECT id, name, created_at FROM shops WHERE account_id=? ORDER BY name, id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	shops := []Shop{}
	for rows.Next() {
		var shop Shop
		if err := rows.Scan(&shop.ID, &shop.Name, &shop.CreatedAt); err != nil {
			return nil, err
		}
		shops = append(shops, shop)
	}
	return shops, rows.Err()
}
