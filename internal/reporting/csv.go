package reporting

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Header is the only metrics template this version imports.
const Header = "source,shop,account,video_id,date,views,orders,gmv_minor,currency,timezone"

var ErrHeader = errors.New("表头与约定模板不一致")

// Row is one video-day fact.
// A nil count is unavailable. A pointer to zero is a real zero.
type Row struct {
	Source   string `json:"source"`
	Shop     string `json:"shop"`
	Account  string `json:"account"`
	VideoID  string `json:"video_id"`
	Date     string `json:"date"`
	Views    *int   `json:"views"`
	Orders   *int   `json:"orders"`
	GMVMinor *int   `json:"gmv_minor"`
	Currency string `json:"currency"`
	Timezone string `json:"timezone"`
	Line     int    `json:"line"`
}

// Key is the idempotency identity: source, shop, account, video, and date.
func (r Row) Key() string {
	return r.Source + "|" + r.Shop + "|" + r.Account + "|" + r.VideoID + "|" + r.Date
}

// RowError is one line that must not be imported.
type RowError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// Preview is what an operator checks before import.
// There is no cross-source total: each row keeps its own source.
type Preview struct {
	Sources    []string   `json:"sources"`
	Shops      []string   `json:"shops"`
	Timezones  []string   `json:"timezones"`
	Currencies []string   `json:"currencies"`
	Rows       []Row      `json:"-"`
	Errors     []RowError `json:"errors"`
	Unbound    []string   `json:"unbound"`
	Accepted   int        `json:"accepted"`
	Repeated   bool       `json:"repeated"`
}

// Inspect parses the agreed CSV template.
// Blank metrics stay unset. The text "0" is zero. Rows without a video ID are rejected.
func Inspect(body []byte) (Preview, error) {
	text := bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
	reader := csv.NewReader(bytes.NewReader(text))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil || len(records) == 0 || strings.Join(trimCells(records[0]), ",") != Header {
		return Preview{}, ErrHeader
	}
	var preview Preview
	for i, record := range records[1:] {
		line := i + 2
		if len(strings.TrimSpace(strings.Join(record, ""))) == 0 {
			continue
		}
		if len(record) != 10 {
			preview.Errors = append(preview.Errors, RowError{Line: line, Message: fmt.Sprintf("第 %d 行列数与模板不一致", line)})
			continue
		}
		cells := trimCells(record)
		row := Row{
			Source: cells[0], Shop: cells[1], Account: cells[2], VideoID: cells[3], Date: cells[4],
			Currency: cells[8], Timezone: cells[9], Line: line,
		}
		if row.Source == "" || row.Shop == "" || row.VideoID == "" || row.Date == "" || row.Currency == "" || row.Timezone == "" {
			preview.Errors = append(preview.Errors, RowError{Line: line, Message: fmt.Sprintf("第 %d 行缺少来源、店铺、视频编号、日期、币种或时区，不能当成视频数据", line)})
			continue
		}
		parsed, err := time.Parse("2006-01-02", row.Date)
		if err != nil || parsed.Format("2006-01-02") != row.Date {
			preview.Errors = append(preview.Errors, RowError{Line: line, Message: fmt.Sprintf("第 %d 行日期格式应为 YYYY-MM-DD", line)})
			continue
		}
		views, err := optionalInt(cells[5])
		if err != nil {
			preview.Errors = append(preview.Errors, RowError{Line: line, Message: fmt.Sprintf("第 %d 行播放量无法读取", line)})
			continue
		}
		orders, err := optionalInt(cells[6])
		if err != nil {
			preview.Errors = append(preview.Errors, RowError{Line: line, Message: fmt.Sprintf("第 %d 行订单数无法读取", line)})
			continue
		}
		gmv, err := optionalInt(cells[7])
		if err != nil {
			preview.Errors = append(preview.Errors, RowError{Line: line, Message: fmt.Sprintf("第 %d 行成交额无法读取", line)})
			continue
		}
		row.Views, row.Orders, row.GMVMinor = views, orders, gmv
		preview.Rows = append(preview.Rows, row)
		preview.Sources = appendUnique(preview.Sources, row.Source)
		preview.Shops = appendUnique(preview.Shops, row.Shop)
		preview.Timezones = appendUnique(preview.Timezones, row.Timezone)
		preview.Currencies = appendUnique(preview.Currencies, row.Currency)
	}
	return preview, nil
}

// Classify drops unbound videos and keys that were already imported.
// It does not add a duplicate into the kept rows.
func Classify(rows []Row, bound map[string]bool, existing map[string]bool) (keep []Row, errs []RowError, unbound []string) {
	seen := map[string]bool{}
	for key, ok := range existing {
		if ok {
			seen[key] = true
		}
	}
	noted := map[string]bool{}
	for _, row := range rows {
		if !bound[row.VideoID] {
			errs = append(errs, RowError{Line: row.Line, Message: fmt.Sprintf("第 %d 行视频还没有绑定成片", row.Line)})
			if !noted[row.VideoID] {
				noted[row.VideoID] = true
				unbound = append(unbound, row.VideoID)
			}
			continue
		}
		if seen[row.Key()] {
			errs = append(errs, RowError{Line: row.Line, Message: fmt.Sprintf("第 %d 行与已有记录重复，没有累计", row.Line)})
			continue
		}
		seen[row.Key()] = true
		keep = append(keep, row)
	}
	return keep, errs, unbound
}

func optionalInt(text string) (*int, error) {
	if text == "" {
		return nil, nil
	}
	value, err := strconv.Atoi(text)
	if err != nil || value < 0 || (len(text) > 1 && text[0] == '0') {
		return nil, errInvalid
	}
	if strconv.Itoa(value) != text {
		return nil, errInvalid
	}
	return &value, nil
}

var errInvalid = errors.New("invalid")

func trimCells(record []string) []string {
	out := make([]string, len(record))
	for i, cell := range record {
		out[i] = strings.TrimSpace(cell)
	}
	return out
}

func appendUnique(values []string, value string) []string {
	for _, have := range values {
		if have == value {
			return values
		}
	}
	return append(values, value)
}
