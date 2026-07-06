package qinglong

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"bilitool-go/internal/cookie"
	"bilitool-go/internal/store"
)

const cookieEnvPrefix = "Ray_BiliBiliCookies__"

type Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	httpClient   *http.Client
}

type Store struct {
	client *Client
}

type Env struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Value   string `json:"value"`
	Remarks string `json:"remarks,omitempty"`
}

type tokenResponse struct {
	Code int       `json:"code"`
	Data tokenData `json:"data"`
}

type tokenData struct {
	TokenType string `json:"token_type"`
	Token     string `json:"token"`
}

type envListResponse struct {
	Code int   `json:"code"`
	Data []Env `json:"data"`
}

type genericResponse struct {
	Code    int    `json:"code"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

type addEnvRequest struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Remarks string `json:"remarks,omitempty"`
}

type updateEnvRequest struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Value   string `json:"value"`
	Remarks string `json:"remarks,omitempty"`
}

func NewClient(baseURL, clientID, clientSecret string) *Client {
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func NewStore(client *Client) *Store {
	return &Store{client: client}
}

func (s *Store) List(ctx context.Context) ([]store.Account, error) {
	token, err := s.client.token(ctx)
	if err != nil {
		return nil, err
	}
	envs, err := s.client.listEnvs(ctx, token)
	if err != nil {
		return nil, err
	}
	accounts := make([]store.Account, 0, len(envs))
	for _, env := range envs {
		if !strings.HasPrefix(env.Name, cookieEnvPrefix) {
			continue
		}
		ck := cookie.FromStringAllowPartial(env.Value)
		accounts = append(accounts, store.Account{
			UID:    ck.UserID(),
			Name:   env.Remarks,
			Cookie: env.Value,
		})
	}
	return accounts, nil
}

func (s *Store) Save(ctx context.Context, account store.Account) error {
	token, err := s.client.token(ctx)
	if err != nil {
		return err
	}
	envs, err := s.client.listEnvs(ctx, token)
	if err != nil {
		return err
	}

	var cookieEnvs []Env
	for _, env := range envs {
		if strings.HasPrefix(env.Name, cookieEnvPrefix) {
			cookieEnvs = append(cookieEnvs, env)
		}
	}

	for _, env := range cookieEnvs {
		ck := cookie.FromStringAllowPartial(env.Value)
		if ck.UserID() == account.UID {
			remarks := env.Remarks
			if remarks == "" {
				remarks = "bili-" + account.UID
			}
			return s.client.updateEnv(ctx, token, updateEnvRequest{
				ID:      env.ID,
				Name:    env.Name,
				Value:   account.Cookie,
				Remarks: remarks,
			})
		}
	}

	return s.client.addEnv(ctx, token, []addEnvRequest{{
		Name:    nextEnvName(cookieEnvs),
		Value:   account.Cookie,
		Remarks: "bili-" + account.UID,
	}})
}

func (c *Client) token(ctx context.Context) (string, error) {
	u, err := url.Parse(c.baseURL + "/open/auth/token")
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", c.clientID)
	q.Set("client_secret", c.clientSecret)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Code != 200 {
		return "", fmt.Errorf("qinglong token failed: code %d", out.Code)
	}
	return out.Data.TokenType + " " + out.Data.Token, nil
}

func (c *Client) listEnvs(ctx context.Context, token string) ([]Env, error) {
	u, err := url.Parse(c.baseURL + "/open/envs")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("searchValue", cookieEnvPrefix)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out envListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Code != 200 {
		return nil, fmt.Errorf("qinglong list envs failed: code %d", out.Code)
	}
	return out.Data, nil
}

func (c *Client) addEnv(ctx context.Context, token string, body []addEnvRequest) error {
	return c.sendJSON(ctx, token, http.MethodPost, "/open/envs", body)
}

func (c *Client) updateEnv(ctx context.Context, token string, body updateEnvRequest) error {
	return c.sendJSON(ctx, token, http.MethodPut, "/open/envs", body)
}

func (c *Client) sendJSON(ctx context.Context, token, method, path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var out genericResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if out.Code != 200 {
		return fmt.Errorf("qinglong %s %s failed: code %d %s", method, path, out.Code, out.Message)
	}
	return nil
}

func nextEnvName(envs []Env) string {
	nums := make([]int, 0, len(envs))
	for _, env := range envs {
		suffix := strings.TrimPrefix(env.Name, cookieEnvPrefix)
		n, err := strconv.Atoi(suffix)
		if err == nil {
			nums = append(nums, n)
		}
	}
	if len(nums) == 0 {
		return cookieEnvPrefix + "0"
	}
	sort.Ints(nums)
	return cookieEnvPrefix + strconv.Itoa(nums[len(nums)-1]+1)
}
