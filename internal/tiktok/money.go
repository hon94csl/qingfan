package tiktok

import (
	"strconv"
	"strings"
)

// MinorUnits converts an official money amount into minor units.
// A blank amount is unset. Zero is a real zero. Extra fractional digits are rejected.
func MinorUnits(amount, currency string) (*int, error) {
	amount = strings.TrimSpace(amount)
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if amount == "" || currency == "" {
		return nil, nil
	}
	scale, ok := isoScale(currency)
	if !ok {
		return nil, errMoney
	}
	whole, frac, _ := strings.Cut(amount, ".")
	if whole == "" || strings.Contains(frac, ".") || strings.ContainsAny(amount, "eE+") {
		return nil, errMoney
	}
	if len(frac) > scale {
		return nil, errMoney
	}
	for len(frac) < scale {
		frac += "0"
	}
	sign := 1
	if strings.HasPrefix(whole, "-") {
		sign = -1
		whole = strings.TrimPrefix(whole, "-")
	}
	if whole == "" || !digits(whole) || !digits(frac) {
		return nil, errMoney
	}
	major, err := strconv.Atoi(whole)
	if err != nil {
		return nil, errMoney
	}
	minor := 0
	if frac != "" {
		minor, err = strconv.Atoi(frac)
		if err != nil {
			return nil, errMoney
		}
	}
	value := sign * (major*pow10(scale) + minor)
	return &value, nil
}

func isoScale(currency string) (int, bool) {
	switch currency {
	case "JPY", "KRW", "VND", "CLP", "ISK", "PYG":
		return 0, true
	case "BHD", "KWD", "OMR":
		return 3, true
	default:
		if len(currency) != 3 || !letters(currency) {
			return 0, false
		}
		return 2, true
	}
}

func digits(s string) bool {
	if s == "" {
		return true
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func letters(s string) bool {
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func pow10(n int) int {
	value := 1
	for i := 0; i < n; i++ {
		value *= 10
	}
	return value
}
