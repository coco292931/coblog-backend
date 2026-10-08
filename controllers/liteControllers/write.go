package liteControllers

import (
	"net/http"
	"strconv"
	"strings"

	"coblog-backend/common/exception"
	"coblog-backend/common/permission"
	"coblog-backend/controllers/liteControllers/liteview"
	"coblog-backend/models"
	"coblog-backend/services/articleService"

	"github.com/gin-gonic/gin"
)

// 写作页（M5）相对主站的降级：
//   - 正文是纯 Markdown textarea，没有实时预览（预览要脚本）
//   - 分类 / 标签是逗号分隔的文本框（主站是 chips 输入）
//   - 封面填 URL（主站的裁剪上传要脚本）
//
// 内容处理与主站一致：只提交 md_content，由后端 goldmark 渲染出 content
// （CreatePost / UpdatePost 的规则就是「Markdown 为单一信源」）。

// requireWritePerm 取账号并校验发帖权限。
// 未登录交给 requireAccount 跳登录页；权限不足跳个人中心提示
// —— 与主站一致，普通用户缺 Perm_PostPost，接口返回 1002。
func requireWritePerm(c *gin.Context) (uint64, bool) {
	accountID, ok := requireAccount(c)
	if !ok {
		return 0, false
	}
	if !hasPerm(c, permission.Perm_PostPost) {
		redirectMe(c, exception.UsrNotPermitted.Msg, false)
		return 0, false
	}
	return accountID, true
}

// loadPostForWrite 按路径里的 id 取文章（含 Markdown 原文）。
// 编辑回填必须用 GetArticleForEdit —— GetArticle 会把 MdContent 清掉。
func loadPostForWrite(c *gin.Context) (models.Post, bool) {
	id := strings.TrimSpace(c.Param("id"))
	if _, err := strconv.ParseUint(id, 10, 64); err != nil {
		renderError(c, http.StatusNotFound)
		return models.Post{}, false
	}
	post, err := articleService.GetArticleForEdit(id)
	if err != nil {
		renderError(c, http.StatusNotFound)
		return models.Post{}, false
	}
	return post, true
}

// WritePage GET /lite/write 新建文章
func WritePage(c *gin.Context) {
	if _, ok := requireWritePerm(c); !ok {
		return
	}
	liteview.Render(c, http.StatusOK, "write", liteview.WriteView{
		BaseView: newBaseView(c),
	})
}

// WriteEditPage GET /lite/write/:id 编辑文章
func WriteEditPage(c *gin.Context) {
	if _, ok := requireWritePerm(c); !ok {
		return
	}
	post, ok := loadPostForWrite(c)
	if !ok {
		return
	}

	liteview.Render(c, http.StatusOK, "write", liteview.WriteView{
		BaseView: newBaseView(c),
		IsEdit:   true,
		ID:       post.ID,
		Title:    post.Title,
		Subtitle: post.Subtitle,
		Summary:  post.Summary,
		Cover:    post.CoverImage,
		Category: liteview.JoinList(liteview.ParseJSONList(post.Category)),
		Tags:     liteview.JoinList(liteview.ParseJSONList(post.Tags)),
		MdBody:   post.MdContent,
		IsDeep:   post.IsDeep,
		Hidden:   post.Hidden,
		NoStats:  post.NoStats,
	})
}

// WriteSubmit POST /lite/write 与 POST /lite/write/:id
//
// 主站用 POST(创建) / PUT(更新) 两个 JSON 接口；表单只能 GET/POST，
// 所以这里按路径里有没有 :id 分派，字段与校验规则照搬。
func WriteSubmit(c *gin.Context) {
	if _, ok := requireWritePerm(c); !ok {
		return
	}

	idStr := strings.TrimSpace(c.Param("id"))
	isEdit := idStr != ""

	view := liteview.WriteView{
		BaseView: newBaseView(c),
		IsEdit:   isEdit,
		Title:    strings.TrimSpace(c.PostForm("title")),
		Subtitle: strings.TrimSpace(c.PostForm("subtitle")),
		Summary:  strings.TrimSpace(c.PostForm("summary")),
		Cover:    strings.TrimSpace(c.PostForm("cover_image")),
		Category: strings.TrimSpace(c.PostForm("category")),
		Tags:     strings.TrimSpace(c.PostForm("tags")),
		MdBody:   c.PostForm("md_content"),
		IsDeep:   c.PostForm("is_deep") != "",
		Hidden:   c.PostForm("hidden") != "",
		NoStats:  c.PostForm("no_stats") != "",
	}

	if isEdit {
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			renderError(c, http.StatusNotFound)
			return
		}
		view.ID = id
	}

	// 校验与后端接口一致：标题非空，且正文非空
	// （主站允许 content / md_content 二选一，这里只有 Markdown 一种输入）
	if view.Title == "" || strings.TrimSpace(view.MdBody) == "" {
		view.Error = exception.ApiParamError.Msg
		liteview.Render(c, http.StatusOK, "write", view)
		return
	}

	fields := articleService.CreatePostInput{
		Title:      view.Title,
		Subtitle:   view.Subtitle,
		Summary:    view.Summary,
		CoverImage: view.Cover,
		MdContent:  view.MdBody,
		Category:   liteview.ToJSONList(view.Category),
		Tags:       liteview.ToJSONList(view.Tags),
		IsDeep:     view.IsDeep,
		Hidden:     view.Hidden,
		NoStats:    view.NoStats,
	}

	var postID uint64
	if isEdit {
		post, err := articleService.UpdatePost(idStr, articleService.UpdatePostInput{
			Title:      fields.Title,
			Subtitle:   fields.Subtitle,
			Summary:    fields.Summary,
			CoverImage: fields.CoverImage,
			Content:    fields.Content,
			MdContent:  fields.MdContent,
			Category:   fields.Category,
			Tags:       fields.Tags,
			IsDeep:     fields.IsDeep,
			Hidden:     fields.Hidden,
			NoStats:    fields.NoStats,
		})
		if err != nil {
			view.Error = errMsg(err)
			liteview.Render(c, http.StatusOK, "write", view)
			return
		}
		postID = post.ID
	} else {
		post, err := articleService.CreatePost(fields)
		if err != nil {
			view.Error = errMsg(err)
			liteview.Render(c, http.StatusOK, "write", view)
			return
		}
		postID = post.ID
	}

	// 隐藏文章在 /lite 里也打不开详情页，所以回列表（主站同样跳文章列表）
	if view.Hidden {
		c.Redirect(http.StatusFound, liteListPath)
		return
	}
	c.Redirect(http.StatusFound, liteListPath+"/"+strconv.FormatUint(postID, 10))
}

// WriteDeleteConfirm GET /lite/write/:id/delete 删除确认
func WriteDeleteConfirm(c *gin.Context) {
	if _, ok := requireWritePerm(c); !ok {
		return
	}
	post, ok := loadPostForWrite(c)
	if !ok {
		return
	}
	liteview.Render(c, http.StatusOK, "confirm-delete", liteview.ConfirmDeleteView{
		BaseView: newBaseView(c),
		ID:       post.ID,
		Title:    post.Title,
	})
}

// WriteDelete POST /lite/write/:id/delete
//
// 主站要求在弹窗里输入完整标题才能确认删除；这里保留同样的要求，
// 只是换成独立确认页 —— 老设备上没有脚本，弹窗做不出来。
func WriteDelete(c *gin.Context) {
	if _, ok := requireWritePerm(c); !ok {
		return
	}
	post, ok := loadPostForWrite(c)
	if !ok {
		return
	}

	if strings.TrimSpace(c.PostForm("confirmTitle")) != post.Title {
		liteview.Render(c, http.StatusOK, "confirm-delete", liteview.ConfirmDeleteView{
			BaseView: newBaseView(c),
			ID:       post.ID,
			Title:    post.Title,
			Error:    "输入的标题与文章标题不一致，请按上方的标题原样填写",
		})
		return
	}

	if err := articleService.DeletePost(strconv.FormatUint(post.ID, 10)); err != nil {
		liteview.Render(c, http.StatusOK, "confirm-delete", liteview.ConfirmDeleteView{
			BaseView: newBaseView(c),
			ID:       post.ID,
			Title:    post.Title,
			Error:    errMsg(err),
		})
		return
	}

	c.Redirect(http.StatusFound, liteListPath)
}
