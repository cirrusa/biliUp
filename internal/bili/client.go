package bili

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"bili-up/internal/cookie"
	"bili-up/internal/wbi"
)

type Options struct {
	APIBaseURL      string
	PassportBaseURL string
	WWWBaseURL      string
	AccountBaseURL  string
	UserAgent       string
	HTTPClient      *http.Client
}

type Client struct {
	apiBase      string
	passportBase string
	wwwBase      string
	accountBase  string
	userAgent    string
	httpClient   *http.Client
}

type Response[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type Empty struct{}

type QRCodeGenerate struct {
	URL       string `json:"url"`
	QRCodeKey string `json:"qrcode_key"`
}

type QRCodePoll struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type WBIImage struct {
	ImgURL string `json:"img_url"`
	SubURL string `json:"sub_url"`
}

type UserInfo struct {
	IsLogin   bool      `json:"isLogin"`
	Uname     string    `json:"uname"`
	Money     float64   `json:"money"`
	LevelInfo LevelInfo `json:"level_info"`
	WBIImage  WBIImage  `json:"wbi_img"`
}

type LevelInfo struct {
	CurrentLevel int `json:"current_level"`
	CurrentExp   int `json:"current_exp"`
}

type DailyReward struct {
	Watch bool `json:"watch"`
	Share bool `json:"share"`
}

type Video struct {
	AID       int64  `json:"aid"`
	BVID      string `json:"bvid"`
	CID       int64  `json:"cid"`
	Title     string `json:"title"`
	Duration  int    `json:"duration"`
	Copyright int    `json:"copyright"`
}

type RankingData struct {
	List []Video `json:"list"`
}

type Page struct {
	Count int `json:"count"`
}

type UPVideoList struct {
	VList []Video `json:"vlist"`
}

type SearchUPVideosData struct {
	Page Page        `json:"page"`
	List UPVideoList `json:"list"`
}

type CoinBalance struct {
	Money float64 `json:"money"`
}

type ArchiveCoins struct {
	Multiply int `json:"multiply"`
}

func NewClient(options Options) *Client {
	if options.APIBaseURL == "" {
		options.APIBaseURL = "https://api.bilibili.com"
	}
	if options.PassportBaseURL == "" {
		options.PassportBaseURL = "http://passport.bilibili.com"
	}
	if options.WWWBaseURL == "" {
		options.WWWBaseURL = "https://www.bilibili.com"
	}
	if options.AccountBaseURL == "" {
		options.AccountBaseURL = "https://account.bilibili.com"
	}
	if options.UserAgent == "" {
		options.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		apiBase:      strings.TrimRight(options.APIBaseURL, "/"),
		passportBase: strings.TrimRight(options.PassportBaseURL, "/"),
		wwwBase:      strings.TrimRight(options.WWWBaseURL, "/"),
		accountBase:  strings.TrimRight(options.AccountBaseURL, "/"),
		userAgent:    options.UserAgent,
		httpClient:   options.HTTPClient,
	}
}

func (c *Client) GenerateQRCode(ctx context.Context) (QRCodeGenerate, error) {
	var out Response[QRCodeGenerate]
	if err := c.getJSON(ctx, c.passportBase+"/x/passport-login/web/qrcode/generate", nil, &out); err != nil {
		return QRCodeGenerate{}, err
	}
	return out.Data, checkCode(out.Code, out.Message)
}

func (c *Client) PollQRCode(ctx context.Context, key string) (*cookie.Cookie, bool, error) {
	u, err := url.Parse(c.passportBase + "/x/passport-login/web/qrcode/poll")
	if err != nil {
		return nil, false, err
	}
	q := u.Query()
	q.Set("qrcode_key", key)
	q.Set("source", "main_mini")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, false, err
	}
	c.setCommonHeaders(req, nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	var out Response[QRCodePoll]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, false, err
	}
	if err := checkCode(out.Code, out.Message); err != nil {
		return nil, false, err
	}
	if out.Data.Code != 0 {
		return nil, false, nil
	}
	ck := cookie.FromStringAllowPartial("")
	ck.MergeSetCookieHeaders(resp.Header.Values("Set-Cookie"))
	parsed, err := cookie.Parse(ck.String())
	if err != nil {
		return nil, false, err
	}
	return parsed, true, nil
}

func (c *Client) SetCookie(ctx context.Context, ck *cookie.Cookie) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.wwwBase, nil)
	if err != nil {
		return err
	}
	c.setCommonHeaders(req, ck)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	cookie.MergeResponseCookies(ck, resp)
	return nil
}

func (c *Client) Nav(ctx context.Context, ck *cookie.Cookie) (UserInfo, error) {
	var out Response[UserInfo]
	if err := c.getJSONWithHeaders(ctx, c.apiBase+"/x/web-interface/nav", ck, browserHeaders("https://www.bilibili.com/", "https://www.bilibili.com"), &out); err != nil {
		return UserInfo{}, err
	}
	if err := checkCode(out.Code, out.Message); err != nil {
		return UserInfo{}, err
	}
	if !out.Data.IsLogin {
		return UserInfo{}, errors.New("Cookie 登录校验失败")
	}
	return out.Data, nil
}

func (c *Client) DailyReward(ctx context.Context, ck *cookie.Cookie) (DailyReward, error) {
	var out Response[DailyReward]
	headers := browserHeaders("https://account.bilibili.com/account/home", "https://account.bilibili.com")
	if err := c.getJSONWithHeaders(ctx, c.apiBase+"/x/member/web/exp/reward", ck, headers, &out); err != nil {
		return DailyReward{}, err
	}
	return out.Data, checkCode(out.Code, out.Message)
}

func (c *Client) Ranking(ctx context.Context) ([]Video, error) {
	var out Response[RankingData]
	headers := browserHeaders("https://www.bilibili.com/v/popular/rank/all", "https://www.bilibili.com")
	headers["DNT"] = "1"
	if err := c.getJSONWithHeaders(ctx, c.apiBase+"/x/web-interface/ranking/v2?rid=0&type=all", nil, headers, &out); err != nil {
		return c.Popular(ctx)
	}
	if err := checkCode(out.Code, out.Message); err != nil {
		return c.Popular(ctx)
	}
	return out.Data.List, nil
}

func (c *Client) Popular(ctx context.Context) ([]Video, error) {
	var out Response[RankingData]
	headers := browserHeaders("https://www.bilibili.com/v/popular/all", "https://www.bilibili.com")
	if err := c.getJSONWithHeaders(ctx, c.apiBase+"/x/web-interface/popular?ps=20&pn=1", nil, headers, &out); err != nil {
		return nil, err
	}
	return out.Data.List, checkCode(out.Code, out.Message)
}

func (c *Client) SearchUPVideos(ctx context.Context, ck *cookie.Cookie, signer wbi.Signer, mid int64, pageNum int) (SearchUPVideosData, error) {
	params := map[string]string{
		"mid":           strconv.FormatInt(mid, 10),
		"ps":            "1",
		"pn":            strconv.Itoa(pageNum),
		"tid":           "0",
		"keyword":       "",
		"order":         "pubdate",
		"platform":      "web",
		"web_location":  "1550101",
		"order_avoided": "true",
	}
	signed := signer.Sign(params, 0)
	u, err := url.Parse(c.apiBase + "/x/space/wbi/arc/search")
	if err != nil {
		return SearchUPVideosData{}, err
	}
	q := u.Query()
	for key, value := range signed {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()

	var out Response[SearchUPVideosData]
	headers := browserHeaders("https://www.bilibili.com/", "https://space.bilibili.com")
	if err := c.getJSONWithHeaders(ctx, u.String(), ck, headers, &out); err != nil {
		return SearchUPVideosData{}, err
	}
	return out.Data, checkCode(out.Code, out.Message)
}

func (c *Client) WatchVideo(ctx context.Context, ck *cookie.Cookie, video Video) error {
	played := 1
	if video.Duration > 1 {
		max := video.Duration
		if max > 15 {
			max = 15
		}
		played = rand.Intn(max) + 1
	}
	form := url.Values{
		"aid":              {strconv.FormatInt(video.AID, 10)},
		"bvid":             {video.BVID},
		"cid":              {strconv.FormatInt(video.CID, 10)},
		"mid":              {ck.UserID()},
		"csrf":             {ck.BiliJct()},
		"played_time":      {strconv.Itoa(played)},
		"realtime":         {strconv.Itoa(played)},
		"real_played_time": {strconv.Itoa(played)},
		"start_ts":         {strconv.FormatInt(time.Now().Unix(), 10)},
		"type":             {"3"},
		"dt":               {"2"},
		"play_type":        {"3"},
	}
	endpoint := fmt.Sprintf("%s/x/click-interface/web/heartbeat?aid=%d&played_time=%d", c.apiBase, video.AID, played)
	return c.postForm(ctx, endpoint, ck, form, "https://www.bilibili.com/")
}

func (c *Client) ShareVideo(ctx context.Context, ck *cookie.Cookie, video Video) error {
	form := url.Values{
		"aid":    {strconv.FormatInt(video.AID, 10)},
		"csrf":   {ck.BiliJct()},
		"eab_x":  {"1"},
		"ramval": {strconv.Itoa(rand.Intn(17) + 3)},
		"source": {"web_normal"},
		"ga":     {"1"},
	}
	return c.postForm(ctx, c.apiBase+"/x/web-interface/share/add", ck, form, "https://www.bilibili.com/")
}

func (c *Client) CoinBalance(ctx context.Context, ck *cookie.Cookie) (float64, error) {
	var out Response[CoinBalance]
	if err := c.getJSONWithHeaders(ctx, c.accountBase+"/site/getCoin", ck, map[string]string{"Referer": "https://account.bilibili.com/account/coin"}, &out); err != nil {
		return 0, err
	}
	return out.Data.Money, checkCode(out.Code, out.Message)
}

func (c *Client) DonateCoinExp(ctx context.Context, ck *cookie.Cookie) (int, error) {
	var out Response[int]
	headers := browserHeaders("https://www.bilibili.com/", "https://www.bilibili.com")
	if err := c.getJSONWithHeaders(ctx, c.apiBase+"/x/web-interface/coin/today/exp", ck, headers, &out); err != nil {
		return 0, err
	}
	return out.Data, checkCode(out.Code, out.Message)
}

func (c *Client) ArchiveCoins(ctx context.Context, ck *cookie.Cookie, aid int64) (ArchiveCoins, error) {
	u, err := url.Parse(c.apiBase + "/x/web-interface/archive/coins")
	if err != nil {
		return ArchiveCoins{}, err
	}
	q := u.Query()
	q.Set("jsonp", "jsonp")
	q.Set("aid", strconv.FormatInt(aid, 10))
	u.RawQuery = q.Encode()
	var out Response[ArchiveCoins]
	if err := c.getJSONWithHeaders(ctx, u.String(), ck, map[string]string{"Referer": "https://www.bilibili.com/"}, &out); err != nil {
		return ArchiveCoins{}, err
	}
	return out.Data, checkCode(out.Code, out.Message)
}

func (c *Client) AddCoin(ctx context.Context, ck *cookie.Cookie, video Video, selectLike bool) error {
	like := "0"
	if selectLike {
		like = "1"
	}
	form := url.Values{
		"aid":          {strconv.FormatInt(video.AID, 10)},
		"multiply":     {"1"},
		"select_like":  {like},
		"cross_domain": {"true"},
		"csrf":         {ck.BiliJct()},
		"eab_x":        {"2"},
		"ramval":       {"3"},
		"source":       {"web_normal"},
		"ga":           {"1"},
	}
	referer := "https://www.bilibili.com/video/" + video.BVID + "/"
	return c.postForm(ctx, c.apiBase+"/x/web-interface/coin/add", ck, form, referer)
}

func SignerFromNav(info UserInfo) wbi.Signer {
	return wbi.NewSigner(keyFromURL(info.WBIImage.ImgURL), keyFromURL(info.WBIImage.SubURL))
}

func keyFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	base := path.Base(u.Path)
	ext := path.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func (c *Client) getJSON(ctx context.Context, endpoint string, ck *cookie.Cookie, out any) error {
	return c.getJSONWithHeaders(ctx, endpoint, ck, nil, out)
}

func (c *Client) getJSONWithHeaders(ctx context.Context, endpoint string, ck *cookie.Cookie, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	c.setCommonHeaders(req, ck)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP GET 请求失败: %s，状态: %s", endpoint, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func browserHeaders(referer, origin string) map[string]string {
	headers := map[string]string{}
	if referer != "" {
		headers["Referer"] = referer
	}
	if origin != "" {
		headers["Origin"] = origin
	}
	return headers
}

func (c *Client) postForm(ctx context.Context, endpoint string, ck *cookie.Cookie, form url.Values, referer string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	c.setCommonHeaders(req, ck)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://www.bilibili.com")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP POST 请求失败: %s，状态: %s", endpoint, resp.Status)
	}
	var out struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	return checkCode(out.Code, out.Message)
}

func (c *Client) setCommonHeaders(req *http.Request, ck *cookie.Cookie) {
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	if ck != nil {
		req.Header.Set("Cookie", ck.String())
	}
}

func checkCode(code int, msg string) error {
	if code == 0 || code == 200 {
		return nil
	}
	if msg == "" {
		msg = "未知错误"
	}
	return fmt.Errorf("哔哩哔哩接口返回错误，code=%d，message=%s", code, msg)
}
