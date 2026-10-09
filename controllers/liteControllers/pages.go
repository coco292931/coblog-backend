// Package liteControllers 实现 /lite（老设备只读页面）的请求处理。
//
// 与主站 SPA 的对应关系：路径层级完全一致，只多一个 /lite 前缀。
// 页面由后端直出 HTML —— 老 Kindle 的浏览器跑不动 Vue 运行时（缺 Proxy /

// ⚠️ 展示项的原则：**只搬运主站已有的，不自己发明**。

package liteControllers

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"coblog-backend/common/exception"
	configreader "coblog-backend/configs/configReader"
	"coblog-backend/controllers/accountControllers"
	"coblog-backend/controllers/liteControllers/liteview"
	"coblog-backend/models"
	"coblog-backend/services/articleService"
	"coblog-backend/services/siteInfoService"
	"coblog-backend/services/userService"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// litePageSize /lite 每页条数。
const litePageSize = 10

// /lite 下与主站对应的路径。
const (
	liteListPath = "/lite/articles"
)

// liteArticleAuthor 版权声明里的作者。

const liteArticleAuthor = "coco_29"

// ────────────────────────────── 页面 ──────────────────────────────

// HomePage GET /lite/
// 对应主站首页：一句欢迎语 + 文章时间线。
func HomePage(c *gin.Context) {
	status := currentContentStatus(c)

	list, err := articleService.GetArticleList(status, articleService.RequestParams{
		Page:     1,
		PageSize: litePageSize,
	}, false)
	if err != nil || list == nil {
		log.Printf("[LITE] 首页取文章列表失败: %v", err)
		renderError(c, http.StatusInternalServerError)
		return
	}

	liteview.Render(c, http.StatusOK, "home", liteview.HomeView{
		BaseView: newBaseView(c),
		Articles: toListItems(list.Articles),
	})
}

// ArticleListPage GET /lite/articles
// 对应主站 /articles 与 /search：支持 page / category / tag / q / sort，
// 全部走查询串（老设备上不依赖任何脚本）。
func ArticleListPage(c *gin.Context) {
	status := currentContentStatus(c)

	keyword := c.Query("q")
	// 关键词长度上限与 /api/articles 一致（binding:"max=100"）。
	// 这里手动截断而不是报错：老设备上更希望看到一个「没搜到」的页面。
	if runes := []rune(keyword); len(runes) > 100 {
		keyword = string(runes[:100])
	}

	params := articleService.RequestParams{
		Page:     liteview.ParsePage(c.Query("page")),
		PageSize: litePageSize,
		Category: c.Query("category"),
		Tag:      c.Query("tag"),
		Q:        keyword,
		Sort:     c.Query("sort"),
	}

	list, err := articleService.GetArticleList(status, params, false)
	if err != nil || list == nil {
		log.Printf("[LITE] 列表页取文章失败: %v", err)
		renderError(c, http.StatusInternalServerError)
		return
	}

	totalPages := uint64(0)
	if list.Total > 0 {
		totalPages = (uint64(list.Total) + litePageSize - 1) / litePageSize
	}

	// 页码越界（例如文章被删掉后旧链接仍指向第 5 页）时收敛到最后一页，
	// 否则会出现「第 5 / 1 页」这种自相矛盾的页面。
	if totalPages > 0 && params.Page > totalPages {
		c.Redirect(http.StatusFound, liteview.BuildPageURL(liteListPath, totalPages, params.Q, params.Category, params.Tag, params.Sort))
		return
	}

	liteview.Render(c, http.StatusOK, "list", liteview.ListView{
		BaseView:   newBaseView(c),
		Articles:   toListItems(list.Articles),
		Total:      list.Total,
		HasFilter:  params.Q != "" || params.Category != "" || params.Tag != "",
		FilterText: liteview.FormatFilterText(params.Q, params.Category, params.Tag),
		Page:       params.Page,
		TotalPages: totalPages,
		HasPrev:    params.Page > 1,
		HasNext:    params.Page < totalPages,
		PrevURL:    liteview.BuildPageURL(liteListPath, liteview.PrevPage(params.Page), params.Q, params.Category, params.Tag, params.Sort),
		NextURL:    liteview.BuildPageURL(liteListPath, params.Page+1, params.Q, params.Category, params.Tag, params.Sort),
		Keyword:    params.Q,
		Category:   params.Category,
		Tag:        params.Tag,

		SortUpdated:      params.Sort == "updated",
		SortPublishedURL: sortURL(false, params),
		SortUpdatedURL:   sortURL(true, params),
	})
}

// ArticlePage GET /lite/articles/:id
// 对应主站文章详情页。可见性（hidden / is_deep）由 articleService 统一保证。
func ArticlePage(c *gin.Context) {
	// 先在入口挡掉非数字 id：直接把它交给 GORM 会得到一个 SQL 层的错误，
	// 对外表现成 500，而这明显属于「地址不对」的情况。
	id, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil {
		renderError(c, http.StatusNotFound)
		return
	}

	post, err := articleService.GetArticle(currentContentStatus(c), strconv.FormatUint(id, 10))
	if err != nil {
		switch {
		case errors.Is(err, exception.UsrNotPermitted):
			// 隐藏文章与深度文章对外等同「不存在」：主站的 404 文案本身就不透露原因。
			renderError(c, http.StatusNotFound)
		case errors.Is(err, gorm.ErrRecordNotFound):
			renderError(c, http.StatusNotFound)
		default:
			log.Printf("[LITE] 文章 %d 读取失败: %v", id, err)
			renderError(c, http.StatusInternalServerError)
		}
		return
	}

	// 版权声明里的「本文链接」指向主站的正式地址（主站用的是当前页 URL）
	base := strings.TrimRight(configreader.GetConfig().Site.BaseURL, "/")

	liteview.Render(c, http.StatusOK, "article", liteview.ArticleView{
		BaseView:   newBaseView(c),
		Title:      post.Title,
		Subtitle:   post.Subtitle,
		Cover:      liteview.ThumbURL(post.CoverImage),
		Content:    template.HTML(liteview.RewriteContentImages(post.Content)),
		CreateTime: liteview.FormatDateTime(post.CreatedAt),
		UpdateTime: liteview.FormatDateTime(post.UpdatedAt),
		Categories: liteview.ParseJSONList(post.Category),
		ReadingMin: liteview.ReadingMinutes(post.Words),
		Author:     liteArticleAuthor,
		ArticleURL: base + "/articles/" + strconv.FormatUint(post.ID, 10),
		Views:      post.Views,
		Likes:      post.Likes,
		// 主站读取的是 data.commentsCount || data.comments_count || 0，
		// 而 models.Post 没有这个字段，所以主站实际也恒显示 0。
		Comments: 0,
	})
}

// AboutPage GET /lite/about、/lite/about/us、/lite/about/friends
// 对应主站 /about 的三个地址，文案与主站保持一致。
func AboutPage(c *gin.Context) {
	tab := "us"
	if strings.HasSuffix(strings.TrimRight(c.Request.URL.Path, "/"), "/friends") {
		tab = "friends"
	}
	liteview.Render(c, http.StatusOK, "about", liteview.AboutView{
		BaseView: newBaseView(c),
		Tab:      tab,
	})
}

// NotFoundPage /lite 下的 404 兜底，由 router 的 NoRoute 调用。
func NotFoundPage(c *gin.Context) {
	renderError(c, http.StatusNotFound)
}

// ServeLiteCSS GET /lite/lite.css
// 转发到视图层，避免 router 直接依赖 liteview。
func ServeLiteCSS(c *gin.Context) {
	liteview.ServeCSS(c)
}

// ────────────────────────────── 辅助 ──────────────────────────────

// currentContentStatus 取当前请求的内容级别。匿名一律为 def。
func currentContentStatus(c *gin.Context) string {
	accountID, err := accountControllers.GetAccountIDFromContext(c)
	if err != nil {
		return userService.ContentStatusDefault
	}
	return userService.ResolveContentStatus(accountID)
}

// newBaseView 组装各页面共用的头部与页脚数据。
//
// 不设置页面标题：主站各路由的 <title> 与导航栏品牌都是 index.html 里的固定字符串。
func newBaseView(c *gin.Context) liteview.BaseView {
	// 游客的 AccountID 是 0（LooseAuth 一定会设置）
	accountID, _ := accountControllers.GetAccountIDFromContext(c)
	return liteview.BaseView{
		// 用 RequestURI 而不是 Path：主站的「当前地址」是含查询串的 fullPath
		CurrentPath: c.Request.URL.RequestURI(),
		NavKey:      liteview.NavKeyFor(c.Request.URL.Path),
		LoggedIn:    accountID != 0,
		Stats:       buildStats(),
	}
}

// buildStats 页脚统计。字段与主站 Footer.vue 一致，换算规则也照搬。
// GetSiteInfo 命中内存缓存，读路径没有额外计算。
func buildStats() *liteview.StatsView {
	info, err := siteInfoService.GetSiteInfo()
	if err != nil {
		return nil
	}
	return &liteview.StatsView{
		TotalWords:  liteview.FormatCompactNumber(info.Words),
		ReadingTime: liteview.FormatSiteReadingTime(info.Words),
		Uptime:      liteview.FormatUptime(info.StartedTime, time.Now()),
	}
}

// renderError 渲染错误页。文案固定用主站的 404 文案，只替换状态码。
func renderError(c *gin.Context, status int) {
	liteview.Render(c, status, "error", liteview.ErrorView{
		BaseView: newBaseView(c),
		Code:     status,
	})
}

// sortURL 生成排序链接：保留当前的筛选条件，回到第 1 页。
func sortURL(updated bool, params articleService.RequestParams) string {
	sort := ""
	if updated {
		sort = "updated"
	}
	return liteview.BuildPageURL(liteListPath, 1, params.Q, params.Category, params.Tag, sort)
}

// toListItems 批量转换列表项。
func toListItems(posts []models.Post) []liteview.ListItemView {
	if len(posts) == 0 {
		return nil
	}
	items := make([]liteview.ListItemView, 0, len(posts))
	for i := range posts {
		items = append(items, toListItem(posts[i]))
	}
	return items
}

// toListItem 把一篇文章转成列表卡片。字段与主站 ArticleTimeline 一致。
func toListItem(p models.Post) liteview.ListItemView {
	return liteview.ListItemView{
		ID:         p.ID,
		Title:      p.Title,
		Summary:    p.Summary,
		Cover:      liteview.ThumbURL(p.CoverImage),
		Date:       liteview.FormatDay(liteview.LatestTime(p)),
		Categories: liteview.ParseJSONList(p.Category),
	}
}
