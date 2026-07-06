package cookie

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type Cookie struct {
	Items map[string]string
}

func Parse(raw string) (*Cookie, error) {
	items := parsePairs(raw)
	ck := &Cookie{Items: items}
	if len(items) == 0 {
		return nil, errors.New("cookie string has no key/value pairs")
	}
	if ck.UserID() == "" {
		return nil, errors.New("cookie missing DedeUserID")
	}
	if ck.SessData() == "" {
		return nil, errors.New("cookie missing SESSDATA")
	}
	if ck.BiliJct() == "" {
		return nil, errors.New("cookie missing bili_jct")
	}
	return ck, nil
}

func FromStringAllowPartial(raw string) *Cookie {
	return &Cookie{Items: parsePairs(raw)}
}

func (c *Cookie) UserID() string {
	return c.Items["DedeUserID"]
}

func (c *Cookie) SessData() string {
	return c.Items["SESSDATA"]
}

func (c *Cookie) BiliJct() string {
	return c.Items["bili_jct"]
}

func (c *Cookie) Buvid3() string {
	return c.Items["buvid3"]
}

func (c *Cookie) String() string {
	keys := make([]string, 0, len(c.Items))
	for key := range c.Items {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, c.Items[key]))
	}
	return strings.Join(parts, "; ")
}

func (c *Cookie) MergeSetCookieHeaders(headers []string) {
	for _, header := range headers {
		if header == "" {
			continue
		}
		first := strings.Split(header, ";")[0]
		idx := strings.Index(first, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(first[:idx])
		value := strings.TrimSpace(first[idx+1:])
		if key != "" {
			c.Items[key] = value
		}
	}
}

func MergeResponseCookies(c *Cookie, resp *http.Response) {
	c.MergeSetCookieHeaders(resp.Header.Values("Set-Cookie"))
}

func parsePairs(raw string) map[string]string {
	items := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.Index(part, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(part[:idx])
		value := strings.TrimSpace(part[idx+1:])
		if key != "" {
			items[key] = value
		}
	}
	return items
}
