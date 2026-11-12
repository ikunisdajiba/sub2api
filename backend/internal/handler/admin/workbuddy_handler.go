package admin

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
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
	h := &WorkBuddyHandler{
		adminService: adminService,
		client:       service.NewWorkBuddyProxyClient(),
	}
	// 反向联动：在 sub2 里停用某个 WorkBuddy 账号时，把网关侧同一个上游号
	// 也停用。否则 sub2 只是"自己不用了"，网关那边仍在把它算进可用池、
	// 别的入口还能继续消耗它——这就不是真的「绑定」。
	service.AccountStatusChangeHook = func(ctx context.Context, account *service.Account, previousStatus string) {
		uid := extractWorkBuddyUID(account.Extra)
		if uid == "" {
			return
		}
		enabled := account.Status == domain.StatusActive
		ctx = service.WithAccountStatusSyncBypass(ctx)
		if _, err := h.client.SetAccount(ctx, map[string]any{"uid": uid, "enabled": enabled}); err != nil {
			slog.Error("workbuddy_reverse_status_sync_failed", "uid", uid, "error", err)
		}
	}
	return h
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
	// 深度融合：把每个 wb 账号对应的 sub2 账号一起带出去，
	// 管理面一眼能看出「这个 wb 号有没有独立账号、当前是否在调度」。
	rows, _ := data["accounts"].([]any)
	links := h.sub2AccountsByUID(c.Request.Context())
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		uid, _ := row["uid"].(string)
		if link, ok := links[strings.TrimSpace(uid)]; ok {
			row["sub2_account"] = link
		}
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
		// 深度融合：网关里停用的账号，sub2 侧的独立账号也要停用，
		// 否则调度器还会往一个已被停用的上游号上打流量。
		if enabled, ok := req["enabled"].(bool); ok {
			h.applyGatewayStateToSub2(ctx, uid, enabled)
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
		// Concurrency 建号时的并发上限；<=0 用默认值。
		Concurrency int `json:"concurrency"`
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
	// 同一个 wb uid 可能对应多条历史账号（早期 sync 每次都会新建）。
	// 这里保留全部而不是互相覆盖，否则去重会漏掉重复项、只更新最后一条，
	// 其余重复账号永远留在库里继续被调度器选中。
	byUID := map[string][]*service.Account{}
	for i := range existing {
		if uid := extractWorkBuddyUID(existing[i].Extra); uid != "" {
			byUID[uid] = append(byUID[uid], &existing[i])
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

		if matches, ok := byUID[uid]; ok && len(matches) > 0 {
			current := matches[0]
			merged := map[string]any{}
			for k, v := range current.Extra {
				merged[k] = v
			}
			for k, v := range extra {
				merged[k] = v
			}
			updateInput := &service.UpdateAccountInput{
				Name:        "WorkBuddy-" + nickname,
				Type:        service.AccountTypeAPIKey,
				Credentials: h.buildCredentials(uid),
				Extra:       merged,
			}
			// 同步时补齐分组：以前只在创建时写分组，已有账号永远补不上，
			// 结果账号在分组外、调度器看不见，等于摆设。
			if len(req.GroupIDs) > 0 {
				groupIDs := req.GroupIDs
				updateInput.GroupIDs = &groupIDs
			}
			if _, err := h.adminService.UpdateAccount(ctx, current.ID, updateInput); err != nil {
				outcome.Errors = append(outcome.Errors, fmt.Sprintf("%s 更新失败: %v", nickname, err))
				continue
			}
			// 重复账号：停用并清掉分组，只保留一条真正在跑的，避免同一上游号
			// 被重复计入池子、并发与限速全部算错。
			for _, dup := range matches[1:] {
				emptyGroups := []int64{}
				_, _ = h.adminService.UpdateAccount(ctx, dup.ID, &service.UpdateAccountInput{
					Status:   domain.StatusDisabled,
					GroupIDs: &emptyGroups,
				})
				outcome.Skipped = append(outcome.Skipped, fmt.Sprintf("%s(重复#%d 已停用)", nickname, dup.ID))
			}
			outcome.Updated = append(outcome.Updated, nickname)
			continue
		}

		// 注意：sub2 的调度器按分组平台挑选账号，只有 platform=openai 的账号
		// 才会被 OpenAI 网关选中。wb 账号的"身份"由 extra.wb_uid +
		// X-WB-Account 请求头锁定，平台仍记为 openai 才能真的跑流量。
		concurrency := req.Concurrency
		if concurrency <= 0 {
			concurrency = 3
		}
		input := &service.CreateAccountInput{
			Name:        "WorkBuddy-" + nickname,
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeAPIKey,
			Credentials: h.buildCredentials(uid),
			Extra:       extra,
			Concurrency: concurrency,
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

// applyGatewayStateToSub2 把网关侧的启停状态同步到对应的 sub2 账号。
// 网关停用 = 上游号不可用，sub2 侧必须同步停用，否则会往死号上打流量。
func (h *WorkBuddyHandler) applyGatewayStateToSub2(ctx context.Context, uid string, enabled bool) {
	accounts, _, err := h.adminService.ListAccounts(ctx, 1, 1000, "", "", "", "", 0, "", "", "")
	if err != nil {
		return
	}
	status := domain.StatusActive
	if !enabled {
		status = domain.StatusDisabled
	}
	for i := range accounts {
		if extractWorkBuddyUID(accounts[i].Extra) != uid {
			continue
		}
		if accounts[i].Status == status {
			continue
		}
		// 用 bypass context：这是网关 -> sub2 的正向同步，
		// 不能再触发 sub2 -> 网关的反向回调，否则两边来回打。
		_, _ = h.adminService.UpdateAccount(service.WithAccountStatusSyncBypass(ctx), accounts[i].ID, &service.UpdateAccountInput{
			Status: status,
		})
	}
}

// sub2AccountsByUID 索引现有 sub2 账号：wb uid -> 账号摘要。
// 一个 wb 账号对应一个独立 sub2 账号，凭据里带 X-WB-Account 锁定流量。
func (h *WorkBuddyHandler) sub2AccountsByUID(ctx context.Context) map[string]gin.H {
	out := map[string]gin.H{}
	existing, _, err := h.adminService.ListAccounts(ctx, 1, 1000, "", "", "", "", 0, "", "", "")
	if err != nil {
		return out
	}
	for i := range existing {
		uid := extractWorkBuddyUID(existing[i].Extra)
		if uid == "" {
			continue
		}
		out[uid] = gin.H{
			"id":          existing[i].ID,
			"name":        existing[i].Name,
			"status":      existing[i].Status,
			"schedulable": existing[i].Schedulable,
			"concurrency": existing[i].Concurrency,
			"priority":    existing[i].Priority,
		}
	}
	return out
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
