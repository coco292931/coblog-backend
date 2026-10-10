package userService

import "coblog-backend/models"

// 内容级别：决定文章列表 / 详情 / RSS 是否包含「深度」文章（is_deep = true）。
//
// 这两个取值同时也是 articleService.GetArticle / GetArticleList 的 status 参数，
// 因此不要在别处另写字面量。
const (
	// ContentStatusDefault 普通访客（未登录，或已登录但不具备深度权限）
	ContentStatusDefault = "def"
	// ContentStatusDeep 具备深度权限
	ContentStatusDeep = "deep"
)

// ResolveContentStatus 由账号 ID 判定内容级别。
//
// 这是原本分散在 articlesControllers（列表 / 详情）与 rssController 三处的
// 同一段逻辑的收口点；新增调用方（如 /lite 只读页面）请直接调用本函数，
// 不要再复制一份判定。
//
// 规则：
//   - 未登录（accountID == 0）→ def
//   - 查不到账号、或账号不同时满足 Deepable 与 IsDeep → def
//   - 同时满足 → deep
//
// 查库失败时降级为 def：宁可少给内容，不可越权。
// 弃用，已由content Status For 替代
func ResolveContentStatus(accountID uint64) string {
	if accountID == 0 {
		return ContentStatusDefault
	}
	account, err := GetUserByID(accountID)
	if err != nil {
		return ContentStatusDefault
	}
	return ContentStatusFor(account)
}

// ContentStatusFor 由已取得的账号信息判定内容级别，
// 供手上已有账号对象、不必再查一次库的调用方使用（如 RSS 的 token 鉴权）。
func ContentStatusFor(account *models.AccountInfo) string {
	if account != nil && account.Deepable && account.IsDeep {
		return ContentStatusDeep
	}
	return ContentStatusDefault
}
