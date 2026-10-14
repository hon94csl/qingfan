package media

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var maxVideoBytes int64 = 200 << 20

// ErrVideoURL means the text is not an http or https URL.
var ErrVideoURL = errors.New("先填写 http 或 https 视频链接")

// ErrVideoUser means the URL carries credentials.
var ErrVideoUser = errors.New("视频链接不能带账号")

// ErrVideoBlocked means the host is not a place this app will fetch.
var ErrVideoBlocked = errors.New("这个链接不能访问")

// ErrVideoDownload means the transfer did not finish. The URL is not included.
var ErrVideoDownload = errors.New("视频链接没有下载完成")

// ErrVideoLarge means the body is over the size cap.
var ErrVideoLarge = errors.New("视频文件过大")

// ParseVideoURL accepts only http and https URLs without user info.
func ParseVideoURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, ErrVideoURL
	}
	if parsed.User != nil {
		return nil, ErrVideoUser
	}
	return parsed, nil
}

// Download writes a remote file to dest. A failed transfer leaves no dest file.
func Download(ctx context.Context, raw, dest string) error {
	parsed, err := ParseVideoURL(raw)
	if err != nil {
		return err
	}
	if err := rejectURL(ctx, parsed); err != nil {
		return err
	}
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ResponseHeaderTimeout: 30 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return ErrVideoDownload
			}
			return rejectURL(req.Context(), req.URL)
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return ErrVideoDownload
	}
	req.Header.Set("User-Agent", "Qingfan")
	resp, err := client.Do(req)
	if err != nil {
		return publicFetchError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ErrVideoDownload
	}
	if resp.ContentLength > maxVideoBytes {
		return ErrVideoLarge
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	partial := dest + ".partial"
	file, err := os.OpenFile(partial, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(file, io.LimitReader(resp.Body, maxVideoBytes+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(partial)
		return ErrVideoDownload
	}
	if written > maxVideoBytes {
		os.Remove(partial)
		return ErrVideoLarge
	}
	if err := os.Rename(partial, dest); err != nil {
		os.Remove(partial)
		return err
	}
	return nil
}

func publicFetchError(err error) error {
	switch {
	case errors.Is(err, ErrVideoURL):
		return ErrVideoURL
	case errors.Is(err, ErrVideoUser):
		return ErrVideoUser
	case errors.Is(err, ErrVideoBlocked):
		return ErrVideoBlocked
	case errors.Is(err, ErrVideoLarge):
		return ErrVideoLarge
	default:
		return ErrVideoDownload
	}
}

func rejectURL(ctx context.Context, parsed *url.URL) error {
	if parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return ErrVideoURL
	}
	if parsed.User != nil {
		return ErrVideoUser
	}
	return rejectHost(ctx, parsed.Hostname())
}

func rejectHost(ctx context.Context, host string) error {
	host = strings.TrimSuffix(host, ".")
	if strings.EqualFold(host, "metadata.google.internal") {
		return ErrVideoBlocked
	}
	if ip := net.ParseIP(host); ip != nil {
		return rejectIP(ip)
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return ErrVideoDownload
	}
	for _, addr := range addrs {
		if err := rejectIP(addr.IP); err != nil {
			return err
		}
	}
	return nil
}

func rejectIP(ip net.IP) error {
	if ip == nil || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return ErrVideoBlocked
	}
	return nil
}
