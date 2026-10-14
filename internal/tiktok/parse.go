package tiktok

import (
	"encoding/json"
	"strconv"
	"strings"
)

type envelope struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

func parseToken(payload json.RawMessage) (Token, error) {
	env, err := decode(payload)
	if err != nil {
		return Token{}, err
	}
	var data struct {
		AccessToken   string   `json:"access_token"`
		AccessExpire  int64    `json:"access_token_expire_in"`
		RefreshToken  string   `json:"refresh_token"`
		RefreshExpire int64    `json:"refresh_token_expire_in"`
		OpenID        string   `json:"open_id"`
		SellerName    string   `json:"seller_name"`
		Region        string   `json:"seller_base_region"`
		UserType      int      `json:"user_type"`
		Scopes        []string `json:"granted_scopes"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return Token{}, err
	}
	if data.AccessToken == "" || data.RefreshToken == "" || data.OpenID == "" {
		return Token{}, errAPI("授权响应缺少令牌")
	}
	if data.Scopes == nil {
		data.Scopes = []string{}
	}
	return Token{
		AccessToken: data.AccessToken, AccessExpire: data.AccessExpire,
		RefreshToken: data.RefreshToken, RefreshExpire: data.RefreshExpire,
		OpenID: data.OpenID, SellerName: data.SellerName, Region: data.Region,
		UserType: data.UserType, Scopes: data.Scopes,
	}, nil
}

func parseShops(payload json.RawMessage) ([]Shop, error) {
	env, err := decode(payload)
	if err != nil {
		return nil, err
	}
	var data struct {
		Shops []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Region     string `json:"region"`
			SellerType string `json:"seller_type"`
			Cipher     string `json:"cipher"`
			Code       string `json:"code"`
		} `json:"shops"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, err
	}
	out := make([]Shop, 0, len(data.Shops))
	for _, shop := range data.Shops {
		if shop.ID == "" || shop.Cipher == "" {
			continue
		}
		out = append(out, Shop{
			ID: shop.ID, Name: shop.Name, Region: shop.Region,
			SellerType: shop.SellerType, Cipher: shop.Cipher, Code: shop.Code,
		})
	}
	return out, nil
}

func parseVideos(payload json.RawMessage) (VideoPage, error) {
	env, err := decode(payload)
	if err != nil {
		return VideoPage{}, err
	}
	var data struct {
		Videos []json.RawMessage `json:"videos"`
		Next   string            `json:"next_page_token"`
		Latest string            `json:"latest_available_date"`
	}
	if len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return VideoPage{}, err
		}
	}
	page := VideoPage{Next: data.Next, Latest: data.Latest}
	for _, raw := range data.Videos {
		video, ok, err := parseVideo(raw)
		if err != nil {
			return VideoPage{}, err
		}
		if ok {
			page.Videos = append(page.Videos, video)
		}
	}
	return page, nil
}

func parseVideo(raw json.RawMessage) (Video, bool, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Video{}, false, err
	}
	id := fieldString(fields["id"])
	if id == "" {
		id = fieldString(fields["video_id"])
	}
	if id == "" {
		return Video{}, false, nil
	}
	video := Video{ID: id, Title: fieldString(fields["title"]), Username: fieldString(fields["username"])}
	if rawViews, ok := fields["views"]; ok {
		views, err := fieldInt(rawViews)
		if err != nil {
			return Video{}, false, err
		}
		video.Views = views
	}
	orderKey := "sku_orders"
	if _, ok := fields[orderKey]; !ok {
		orderKey = "orders"
	}
	if rawOrders, ok := fields[orderKey]; ok {
		orders, err := fieldInt(rawOrders)
		if err != nil {
			return Video{}, false, err
		}
		video.Orders = orders
	}
	if rawGMV, ok := fields["gmv"]; ok && string(rawGMV) != "null" && strings.HasPrefix(strings.TrimSpace(string(rawGMV)), "{") {
		var money struct {
			Amount   string `json:"amount"`
			Currency string `json:"currency"`
		}
		if err := json.Unmarshal(rawGMV, &money); err != nil {
			return Video{}, false, errMoney
		}
		minor, err := MinorUnits(money.Amount, money.Currency)
		if err != nil {
			return Video{}, false, err
		}
		video.GMVMinor = minor
		if minor != nil {
			video.Currency = strings.ToUpper(strings.TrimSpace(money.Currency))
		}
	}
	return video, true, nil
}

func decode(payload json.RawMessage) (envelope, error) {
	var env envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return envelope{}, err
	}
	if env.Code != 0 {
		message := strings.TrimSpace(env.Message)
		if message == "" {
			message = "请求没有成功"
		}
		return envelope{}, errAPI(message)
	}
	return env, nil
}

func errAPI(message string) error {
	return &APIError{message: message}
}

// APIError is a failed official response. The message is the API text, not the response body.
type APIError struct{ message string }

func (e *APIError) Error() string { return "TikTok Shop：" + e.message }

func fieldString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return number.String()
	}
	return ""
}

func fieldInt(raw json.RawMessage) (*int, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return nil, errMoney
		}
		number = json.Number(strings.TrimSpace(text))
	}
	text := number.String()
	if text == "" || strings.Contains(text, ".") || strings.ContainsAny(text, "eE") {
		return nil, errMoney
	}
	value, err := strconv.Atoi(text)
	if err != nil {
		return nil, errMoney
	}
	return &value, nil
}
