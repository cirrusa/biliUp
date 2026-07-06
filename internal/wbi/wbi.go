package wbi

import (
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Signer struct {
	imgKey string
	subKey string
}

var mixinKeyEncTab = []int{
	46, 47, 18, 2, 53, 8, 23, 32,
	15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19,
	29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61,
	26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63,
	57, 62, 11, 36, 20, 34, 44, 52,
}

var chrFilter = regexp.MustCompile(`[!'()*]`)

func NewSigner(imgKey, subKey string) Signer {
	return Signer{imgKey: imgKey, subKey: subKey}
}

func (s Signer) Sign(params map[string]string, ts int64) map[string]string {
	if ts == 0 {
		ts = time.Now().Unix()
	}
	out := make(map[string]string, len(params)+2)
	for k, v := range params {
		if k == "w_rid" || k == "wts" {
			continue
		}
		out[k] = v
	}
	out["wts"] = strconv.FormatInt(ts, 10)

	keys := make([]string, 0, len(out))
	for key := range out {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := chrFilter.ReplaceAllString(out[key], "")
		parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(value))
	}
	query := strings.Join(parts, "&")
	sum := md5.Sum([]byte(query + s.mixinKey()))
	out["w_rid"] = hex.EncodeToString(sum[:])
	return out
}

func (s Signer) mixinKey() string {
	orig := s.imgKey + s.subKey
	var b strings.Builder
	for _, idx := range mixinKeyEncTab {
		if idx >= 0 && idx < len(orig) {
			b.WriteByte(orig[idx])
		}
	}
	mixed := b.String()
	if len(mixed) > 32 {
		return mixed[:32]
	}
	return mixed
}
