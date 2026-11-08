package admin

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// WorkBuddyHandler 把 WorkBuddy 反代网关（wb-proxy）的账号管理融合进 sub2api 管理面。
type WorkBuddyHandler struct {
	adminService service.AdminService
	client       *service.WorkBuddyProxyClient
}

// NewWorkBuddyHandler 创建 WorkBuddy 网关管理处理器。
func NewWorkBuddyHandler(adminService service.AdminService) *WorkBuddyHandler {
	return &WorkBuddyHandler{
		adminService: adminService,
		client:       service.NewWorkBuddyProxyClient(),
	}
}

func (h *WorkBuddyHandler) guard(c *gin.Context) bool {
	if !h.client.Configured() {
		response.Error(c, http.StatusServiceUnavailable, "WorkBuddy 网关未配置：请设置 WB_PROXY_URL / WB_PANEL_PASSWORD")
		return false
	}
	return true
}

// ListAccounts GET /admin/workbuddy/accounts?realm=all
func (h *WorkBuddyHandler) ListAccounts(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	realm := strings.TrimSpace(c.DefaultQuery("realm", "all"))
	data, err := h.client.Accounts(c.Request.Context(), realm)
	if err != nil {
		response.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	response.Success(c, gin.H{
		"accounts":     data["accounts"],
		"usable":       data["usable"],
		"realm":        realm,
		"base_url":     h.client.BaseURL(),
		"configured":   true,
		"upstream_key": h.client.UpstreamAPIKey() != "",
	})
}

// LoginStart POST /admin/workbuddy/login/start
func (h *WorkBuddyHandler) LoginStart(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	var req struct {
		Realm    string `json:"realm"`
		Platform string `json:"platform"`
	}
	_ = c.ShouldBindJSON(&req)
	data, err := h.client.LoginStart(c.Request.Context(), req.Realm, req.Platform)
	if err != nil {
		response.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	response.Success(c, data)
}

// LoginPoll GET /admin/workbuddy/login/poll?state=xxx
func (h *WorkBuddyHandler) LoginPoll(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	state := strings.TrimSpace(c.Query("state"))
	if state == "" {
		response.BadRequest(c, "state 必填")
		return
	}
	data, err := h.client.LoginPoll(c.Request.Context(), state)
	if err != nil {
		response.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	response.Success(c, data)
}

// Checkin POST /admin/workbuddy/accounts/checkin
func (h *WorkBuddyHandler) Checkin(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	var req struct {
		UID string `json:"uid"`
	}
	_ = c.ShouldBindJSON(&req)
	data, err := h.client.Checkin(c.Request.Context(), strings.TrimSpace(req.UID))
	if err != nil {
		response.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	response.Success(c, data)
}

// FetchCredits POST /admin/workbuddy/accounts/credits
func (h *WorkBuddyHandler) FetchCredits(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	var req struct {
		UID string `json:"uid"`
	}
	_ = c.ShouldBindJSON(&req)
	data, err := h.client.FetchCredits(c.Request.Context(), strings.TrimSpace(req.UID))
	if err != nil {
		response.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	response.Success(c, data)
}

// UpdateAccount POST /admin/workbuddy/accounts/update
func (h *WorkBuddyHandler) UpdateAccount(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	var req map[string]any
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid, _ := req["uid"].(string)
	if strings.TrimSpace(uid) == "" {
		response.BadRequest(c, "uid 必填")
		return
	}
	ctx := c.Request.Context()
	if product, ok := req["product"].(string); ok && strings.TrimSpace(product) != "" {
		if _, err := h.client.SetProduct(ctx, uid, product); err != nil {
			response.Error(c, http.StatusBadGateway, err.Error())
			return
		}
		delete(req, "product")
	}
	if len(req) > 1 {
		if _, err := h.client.SetAccount(ctx, req); err != nil {
			response.Error(c, http.StatusBadGateway, err.Error())
			return
		}
	}
	data, err := h.client.Accounts(ctx, "all")
	if err != nil {
		response.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	response.Success(c, data)
}

// DeleteAccount POST /admin/workbuddy/accounts/delete
func (h *WorkBuddyHandler) DeleteAccount(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	var req struct {
		UID string `json:"uid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := strings.TrimSpace(req.UID)
	if uid == "" {
		response.BadRequest(c, "uid 必填")
		return
	}
	ctx := c.Request.Context()
	if _, err := h.client.DeleteAccount(ctx, uid); err != nil {
		response.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	// 网关删了，把对应的 sub2 账号也删掉，避免留下死号。
	if accounts, _, err := h.adminService.ListAccounts(ctx, 1, 1000, "", "", "", "", 0, "", "", ""); err == nil {
		for i := range accounts {
			if extractWorkBuddyUID(accounts[i].Extra) == uid {
				_ = h.adminService.DeleteAccount(ctx, accounts[i].ID)
			}
		}
	}
	data, err := h.client.Accounts(ctx, "all")
	if err != nil {
		response.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	response.Success(c, data)
}

// Sync POST /admin/workbuddy/sync
// 把网关里的真实账号同步成 sub2 账号：每个 wb 账号对应一个 sub2 账号，
// 凭据里带 X-WB-Account 覆写，流量锁定到该上游账号。
func (h *WorkBuddyHandler) Sync(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	var req struct {
		UIDs     []string `json:"uids"`
		Realm    string   `json:"realm"`
		GroupIDs []int64  `json:"group_ids"`
	}
	_ = c.ShouldBindJSON(&req)

	ctx := c.Request.Context()
	if strings.TrimSpace(req.Realm) == "" {
		req.Realm = "all"
	}
	data, err := h.client.Accounts(ctx, req.Realm)
	if err != nil {
		response.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	rows, _ := data["accounts"].([]any)

	wanted := map[string]struct{}{}
	for _, uid := range req.UIDs {
		if uid = strings.TrimSpace(uid); uid != "" {
			wanted[uid] = struct{}{}
		}
	}

	existing, _, err := h.adminService.ListAccounts(ctx, 1, 1000, "", "", "", "", 0, "", "", "")
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	byUID := map[string]*service.Account{}
	for i := range existing {
		if uid := extractWorkBuddyUID(existing[i].Extra); uid != "" {
			byUID[uid] = &existing[i]
		}
	}

	type syncOutcome struct {
		Created []string `json:"created"`
		Updated []string `json:"updated"`
		Skipped []string `json:"skipped"`
		Errors  []string `json:"errors"`
	}
	outcome := syncOutcome{Created: []string{}, Updated: []string{}, Skipped: []string{}, Errors: []string{}}

	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		uid, _ := row["uid"].(string)
		uid = strings.TrimSpace(uid)
		if uid == "" {
			continue
		}
		if len(wanted) > 0 {
			if _, ok := wanted[uid]; !ok {
				continue
			}
		}

		nickname, _ := row["nickname"].(string)
		if strings.TrimSpace(nickname) == "" {
			nickname = uid
			if len(uid) > 8 {
				nickname = uid[:8]
			}
		}
		extra := buildWorkBuddyExtra(uid, row)

		if current, ok := byUID[uid]; ok {
			merged := map[string]any{}
			for k, v := range current.Extra {
				merged[k] = v
			}
			for k, v := range extra {
				merged[k] = v
			}
			input := &service.UpdateAccountInput{
				Name:        "WorkBuddy-" + nickname,
				Type:        service.AccountTypeAPIKey,
				Credentials: h.buildCredentials(uid),
				Extra:       merged,
			}
			if _, err := h.adminService.UpdateAccount(ctx, current.ID, input); err != nil {
				outcome.Errors = append(outcome.Errors, fmt.Sprintf("%s 更新失败: %v", nickname, err))
				continue
			}
			outcome.Updated = append(outcome.Updated, nickname)
			continue
		}

		// 注意：sub2 的调度器按分组平台挑选账号，只有 platform=openai 的账号
		// 才会被 OpenAI 网关选中。wb 账号的"身份"由 extra.wb_uid +
		// X-WB-Account 请求头锁定，平台仍记为 openai 才能真的跑流量。
		input := &service.CreateAccountInput{
			Name:        "WorkBuddy-" + nickname,
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeAPIKey,
			Credentials: h.buildCredentials(uid),
			Extra:       extra,
			Concurrency: 3,
			Priority:    1,
			GroupIDs:    req.GroupIDs,
		}
		if _, err := h.adminService.CreateAccount(ctx, input); err != nil {
			outcome.Errors = append(outcome.Errors, fmt.Sprintf("%s 创建失败: %v", nickname, err))
			continue
		}
		outcome.Created = append(outcome.Created, nickname)
	}

	response.Success(c, gin.H{
		"result":   outcome,
		"accounts": data["accounts"],
	})
}

func (h *WorkBuddyHandler) buildCredentials(uid string) map[string]any {
	creds := map[string]any{
		"base_url":                h.client.BaseURL(),
		"header_override_enabled": true,
		"header_overrides": map[string]any{
			"X-WB-Account": uid,
		},
	}
	if key := h.client.UpstreamAPIKey(); key != "" {
		creds["api_key"] = key
	}
	return creds
}

func extractWorkBuddyUID(extra map[string]any) string {
	if extra == nil {
		return ""
	}
	uid, _ := extra["wb_uid"].(string)
	return strings.TrimSpace(uid)
}

func buildWorkBuddyExtra(uid string, row map[string]any) map[string]any {
	extra := map[string]any{
		"wb_uid":       uid,
		"wb_synced_at": time.Now().Format(time.RFC3339),
	}
	for _, key := range []string{"nickname", "realm", "product", "platform", "enabled", "weight", "note", "domain", "expiresAt", "lastError"} {
		if v, ok := row[key]; ok {
			extra["wb_"+key] = v
		}
	}
	if credits, ok := row["credits"]; ok && credits != nil {
		extra["wb_credits"] = map[string]any{
			"pool":       credits,
			"synced_at":  time.Now().Format(time.RFC3339),
			"account_id": uid,
		}
	}
	return extra
}