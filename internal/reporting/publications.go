package reporting

import (
	"errors"
	"strings"
)

var (
	ErrOutput = errors.New("找不到这条成片，不能绑定发布")
	ErrVideo  = errors.New("视频编号不能为空")
	ErrShop   = errors.New("店铺不能为空")
	ErrWhen   = errors.New("发布时间不能为空")
)

// Publication binds one TikTok post to an immutable output.
// One output may have several publications. The output file is not changed here.
type Publication struct {
	ID          string `json:"id"`
	OutputID    string `json:"output_id"`
	URL         string `json:"url"`
	VideoID     string `json:"video_id"`
	Shop        string `json:"shop"`
	PublishedAt string `json:"published_at"`
}

// Bind checks a hand-entered post.
// A full video ID is enough when the short link cannot be resolved.
func Bind(outputID, url, videoID, shop, publishedAt string, outputExists bool) (Publication, error) {
	if strings.TrimSpace(outputID) == "" || !outputExists {
		return Publication{}, ErrOutput
	}
	videoID = strings.TrimSpace(videoID)
	shop = strings.TrimSpace(shop)
	publishedAt = strings.TrimSpace(publishedAt)
	if videoID == "" {
		return Publication{}, ErrVideo
	}
	if shop == "" {
		return Publication{}, ErrShop
	}
	if publishedAt == "" {
		return Publication{}, ErrWhen
	}
	return Publication{
		OutputID:    outputID,
		URL:         strings.TrimSpace(url),
		VideoID:     videoID,
		Shop:        shop,
		PublishedAt: publishedAt,
	}, nil
}
