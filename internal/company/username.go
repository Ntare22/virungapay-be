package company

import (
	"strings"
	"unicode"
)

func usernameSlug(base string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(base)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "wallet"
	}
	if len(slug) > 20 {
		slug = strings.Trim(slug[:20], "-")
	}
	return slug
}

func usernameCandidate(base, currency string, n int) string {
	slug := usernameSlug(base) + "-" + strings.ToLower(strings.TrimSpace(currency))
	if n <= 1 {
		return slug
	}
	return slug + "-" + strings.TrimSpace(strings.ToLower(itoa(n)))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [12]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}

func normalizeUsername(raw string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(raw)), "@")
}

// chosenUsername turns what a person typed into the handle senders will use.
func chosenUsername(raw string) (string, bool) {
	var b strings.Builder
	dash := false
	letters := 0
	for _, r := range strings.ToLower(strings.TrimPrefix(strings.TrimSpace(raw), "@")) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
			letters++
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	username := strings.Trim(b.String(), "-")
	if letters < 3 || len(username) > 30 {
		return "", false
	}
	return username, true
}
