package tiktok

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	AuthBase   = "https://auth.tiktok-shops.com"
	APIBase    = "https://open-api.tiktokglobalshop.com"
	ShopsPath  = "/authorization/202309/shops"
	VideosPath = "/analytics/202605/shop_videos/performance"
)

var (
	errMoney     = errors.New("成交额无法换算")
	ErrSeller    = errors.New("这不是卖家授权")
	ErrConfig    = errors.New("还没有配置 TikTok Shop 应用")
	ErrState     = errors.New("授权状态对不上")
	ErrShop      = errors.New("找不到这家店铺")
	ErrRange     = errors.New("日期区间应为 YYYY-MM-DD，结束日不包含在内")
	ErrPartial   = errors.New("官方接口还有下一页，这次没有写入不完整的结果")
	ErrCode      = errors.New("授权码不能为空")
	ErrNoShop    = errors.New("这次授权没有返回店铺")
	ErrOwned     = errors.New("这家店铺已经由其他账号接入")
	errTransport = errors.New("TikTok Shop 请求没有完成")
)

// Client calls the TikTok Shop Open API. Tests replace the bases and HTTP client.
type Client struct {
	AppKey    string
	AppSecret string
	ServiceID string
	US        bool
	HTTP      *http.Client
	Now       func() time.Time
	AuthBase  string
	APIBase   string
}

// Configured reports whether the Partner Center app credentials are present.
func (c *Client) Configured() bool {
	return c != nil && c.AppKey != "" && c.AppSecret != "" && c.ServiceID != ""
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c *Client) now() time.Time {
	if c != nil && c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Moment is the clock used for signatures and token refresh.
func (c *Client) Moment() time.Time { return c.now() }

// ClientFromEnv reads the Partner Center app. A missing value stays empty so the caller can report it.
func ClientFromEnv() *Client {
	market := os.Getenv("QINGFAN_TIKTOK_MARKET")
	if market == "" {
		market = os.Getenv("QINGFAN_TIKTOK_REGION")
	}
	return &Client{
		AppKey:    os.Getenv("QINGFAN_TIKTOK_APP_KEY"),
		AppSecret: os.Getenv("QINGFAN_TIKTOK_APP_SECRET"),
		ServiceID: os.Getenv("QINGFAN_TIKTOK_SERVICE_ID"),
		US:        strings.EqualFold(market, "US"),
	}
}

func (c *Client) authBase() string {
	if c.AuthBase != "" {
		return c.AuthBase
	}
	return AuthBase
}

func (c *Client) apiBase() string {
	if c.APIBase != "" {
		return c.APIBase
	}
	return APIBase
}

// AuthorizeURL is the seller authorization link. US uses the US host. Other markets, including ID, use the rest-of-world host.
func (c *Client) AuthorizeURL(state string) (string, error) {
	if !c.Configured() {
		return "", ErrConfig
	}
	host := "https://services.tiktokshop.com/open/authorize"
	if c.US {
		host = "https://services.us.tiktokshop.com/open/authorize"
	}
	values := url.Values{"service_id": {c.ServiceID}}
	if state != "" {
		values.Set("state", state)
	}
	return host + "?" + values.Encode(), nil
}

// Token is a seller access grant. The plaintext tokens stay on the server.
type Token struct {
	AccessToken   string
	AccessExpire  int64
	RefreshToken  string
	RefreshExpire int64
	OpenID        string
	SellerName    string
	Region        string
	UserType      int
	Scopes        []string
}

// Shop is one authorized shop. Cipher is required by later shop calls and is not shown to the browser.
type Shop struct {
	ID         string
	Name       string
	Region     string
	SellerType string
	Cipher     string
	Code       string
}

// Video is one shop video over a requested date range.
// A nil count is absent. A pointer to zero is a real zero.
type Video struct {
	ID       string
	Title    string
	Username string
	Views    *int
	Orders   *int
	GMVMinor *int
	Currency string
}

// VideoPage is one page of Get Shop Video Performance.
type VideoPage struct {
	Videos []Video
	Next   string
	Latest string
}

// Exchange turns an authorization code into a seller token.
func (c *Client) Exchange(ctx context.Context, code string) (Token, error) {
	return c.token(ctx, url.Values{
		"app_key":    {c.AppKey},
		"app_secret": {c.AppSecret},
		"auth_code":  {code},
		"grant_type": {"authorized_code"},
	})
}

// Refresh renews an access token.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	return c.token(ctx, url.Values{
		"app_key":       {c.AppKey},
		"app_secret":    {c.AppSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	})
}

func (c *Client) token(ctx context.Context, query url.Values) (Token, error) {
	if !c.Configured() {
		return Token{}, ErrConfig
	}
	path := "/api/v2/token/get"
	if query.Get("grant_type") == "refresh_token" {
		path = "/api/v2/token/refresh"
	}
	endpoint := c.authBase() + path + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Token{}, err
	}
	body, err := c.do(req)
	if err != nil {
		return Token{}, err
	}
	token, err := parseToken(body)
	if err != nil {
		return Token{}, err
	}
	if token.UserType != 0 {
		return Token{}, ErrSeller
	}
	return token, nil
}

// Shops lists the shops authorized for the seller token.
func (c *Client) Shops(ctx context.Context, accessToken string) ([]Shop, error) {
	body, err := c.signed(ctx, http.MethodGet, ShopsPath, accessToken, url.Values{}, "")
	if err != nil {
		return nil, err
	}
	return parseShops(body)
}

// Videos reads one page of shop video performance. Dates are inclusive start and exclusive end, in the shop timezone.
func (c *Client) Videos(ctx context.Context, accessToken, cipher, start, end, pageToken string) (VideoPage, error) {
	query := url.Values{
		"shop_cipher":   {cipher},
		"start_date_ge": {start},
		"end_date_lt":   {end},
		"page_size":     {"100"},
		"account_type":  {"ALL"},
	}
	if pageToken != "" {
		query.Set("page_token", pageToken)
	}
	body, err := c.signed(ctx, http.MethodGet, VideosPath, accessToken, query, "")
	if err != nil {
		return VideoPage{}, err
	}
	return parseVideos(body)
}

func (c *Client) signed(ctx context.Context, method, path, accessToken string, query url.Values, body string) (json.RawMessage, error) {
	if !c.Configured() {
		return nil, ErrConfig
	}
	query.Set("app_key", c.AppKey)
	query.Set("timestamp", strconv.FormatInt(c.now().Unix(), 10))
	query.Set("sign", Sign(c.AppSecret, path, query, body))
	endpoint := c.apiBase() + path + "?" + query.Encode()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-tts-access-token", accessToken)
	return c.do(req)
}

func (c *Client) do(req *http.Request) (json.RawMessage, error) {
	res, err := c.httpClient().Do(req)
	if err != nil {
		return nil, errTransport
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("TikTok Shop 响应 %d", res.StatusCode)
	}
	return payload, nil
}
