package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// WorkBuddyProxyClient 访问 WorkBuddy 反代网关（wb-proxy）的管理接口。
// 网关的 /accounts 等管理路由要求 X-Panel-Token，由面板密码换发。
// 上游请求用 WB_UPSTREAM_API_KEY，配合账号级请求头 X-WB-Account 锁定到具体上游账号。
type WorkBuddyProxyClient struct {
	baseURL  string
	password string
	apiKey   string
	http     *http.Client

	mu    sync.Mutex
	token string
}

// NewWorkBuddyProxyClient 从环境变量构造客户端。
func NewWorkBuddyProxyClient() *WorkBuddyProxyClient {
	base := strings.TrimSpace(os.Getenv("WB_PROXY_URL"))
	if base == "" {
		base = "http://wb-proxy:8788"
	}
	return &WorkBuddyProxyClient{
		baseURL:  strings.TrimRight(base, "/"),
		password: strings.TrimSpace(os.Getenv("WB_PANEL_PASSWORD")),
		apiKey:   strings.TrimSpace(os.Getenv("WB_UPSTREAM_API_KEY")),
		http:     &http.Client{Timeout: 90 * time.Second},
	}
}

// BaseURL 返回网关地址（同时作为账号的 base_url）。
func (c *WorkBuddyProxyClient) BaseURL() string { return c.baseURL }

// UpstreamAPIKey 返回转发到网关用的 API Key。
func (c *WorkBuddyProxyClient) UpstreamAPIKey() string { return c.apiKey }

// Configured 报告管理接口是否已配置（需要面板密码）。
func (c *WorkBuddyProxyClient) Configured() bool { return c.password != "" }

func (c *WorkBuddyProxyClient) invalidateToken() {
	c.mu.Lock()
	c.token = ""
	c.mu.Unlock()
}

func (c *WorkBuddyProxyClient) login(ctx context.Context) (string, error) {
	if c.password == "" {
		return "", errors.New("WB_PANEL_PASSWORD 未配置，无法调用 WorkBuddy 网关管理接口")
	}
	c.mu.Lock()
	token := c.token
	c.mu.Unlock()
	if token != "" {
		return token, nil
	}
	res, err := c.call(ctx, http.MethodPost, "/panel/login", map[string]any{"password": c.password}, false)
	if err != nil {
		return "", err
	}
	token, _ = res["token"].(string)
	if token == "" {
		return "", errors.New("WorkBuddy 网关登录未返回 token")
	}
	c.mu.Lock()
	c.token = token
	c.mu.Unlock()
	return token, nil
}

func (c *WorkBuddyProxyClient) call(ctx context.Context, method, path string, payload any, authed bool) (map[string]any, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		token := ""
		if authed {
			t, err := c.login(ctx)
			if err != nil {
				return nil, err
			}
			token = t
		}
		var body io.Reader
		if payload != nil {
			buf, err := json.Marshal(payload)
			if err != nil {
				return nil, err
			}
			body = bytes.NewReader(buf)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("X-Panel-Token", token)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if authed && resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			c.invalidateToken()
			lastErr = errors.New("WorkBuddy 网关会话失效，已重试")
			continue
		}

		out := map[string]any{}
		if len(bytes.TrimSpace(raw)) > 0 {
			if err := json.Unmarshal(raw, &out); err != nil {
				if resp.StatusCode >= 400 {
					return nil, fmt.Errorf("WorkBuddy 网关 %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
				}
				return nil, fmt.Errorf("WorkBuddy 网关返回了非 JSON 响应: %w", err)
			}
		}
		if resp.StatusCode >= 400 {
			detail := ""
			if e, ok := out["error"].(map[string]any); ok {
				detail, _ = e["message"].(string)
			}
			if detail == "" {
				detail = strings.TrimSpace(string(raw))
			}
			return out, fmt.Errorf("WorkBuddy 网关 %d: %s", resp.StatusCode, detail)
		}
		return out, nil
	}
	if lastErr == nil {
		lastErr = errors.New("WorkBuddy 网关请求失败")
	}
	return nil, lastErr
}

// Accounts 列出网关里的真实账号（realm: all / cn / intl）。
func (c *WorkBuddyProxyClient) Accounts(ctx context.Context, realm string) (map[string]any, error) {
	if strings.TrimSpace(realm) == "" {
		realm = "all"
	}
	return c.call(ctx, http.MethodGet, "/accounts?realm="+realm, nil, true)
}

// LoginStart 发起一次上游账号登录，返回 state 与 authUrl。
func (c *WorkBuddyProxyClient) LoginStart(ctx context.Context, realm, platform string) (map[string]any, error) {
	payload := map[string]any{
		"realm":    strings.TrimSpace(realm),
		"platform": strings.TrimSpace(platform),
	}
	if payload["realm"] == "" {
		payload["realm"] = "cn"
	}
	if payload["platform"] == "" {
		payload["platform"] = "CLI"
	}
	return c.call(ctx, http.MethodPost, "/accounts/login/start", payload, true)
}

// LoginPoll 查询登录进度。
func (c *WorkBuddyProxyClient) LoginPoll(ctx context.Context, state string) (map[string]any, error) {
	return c.call(ctx, http.MethodGet, "/accounts/login/poll?state="+state, nil, true)
}

// Checkin 触发签到（uid 为空则全部 cn 账号）。
func (c *WorkBuddyProxyClient) Checkin(ctx context.Context, uid string) (map[string]any, error) {
	return c.call(ctx, http.MethodPost, "/accounts/checkin", map[string]any{"uid": uid}, true)
}

// FetchCredits 从上游刷新积分。
func (c *WorkBuddyProxyClient) FetchCredits(ctx context.Context, uid string) (map[string]any, error) {
	payload := map[string]any{"realm": "all"}
	if uid != "" {
		payload = map[string]any{"uid": uid}
	}
	return c.call(ctx, http.MethodPost, "/accounts/credits", payload, true)
}

// SetAccount 修改账号（enabled / weight / note / proxy / proxySlot / product）。
func (c *WorkBuddyProxyClient) SetAccount(ctx context.Context, payload map[string]any) (map[string]any, error) {
	return c.call(ctx, http.MethodPost, "/accounts/set", payload, true)
}

// SetProduct 切换账号出站身分（cli / workbuddy / vscode）。
func (c *WorkBuddyProxyClient) SetProduct(ctx context.Context, uid, product string) (map[string]any, error) {
	return c.call(ctx, http.MethodPost, "/accounts/product", map[string]any{"uid": uid, "product": product}, true)
}

// DeleteAccount 从网关删除账号。
func (c *WorkBuddyProxyClient) DeleteAccount(ctx context.Context, uid string) (map[string]any, error) {
	return c.call(ctx, http.MethodPost, "/accounts/delete", map[string]any{"uid": uid}, true)
}