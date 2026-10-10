package liteControllers

import (
	"errors"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"coblog-backend/common/exception"
	"coblog-backend/common/permission"
	"coblog-backend/controllers/fileController"
	"coblog-backend/controllers/liteControllers/liteview"
	"coblog-backend/models"
	"coblog-backend/services/articleService"
	"coblog-backend/utils"

	"github.com/gin-gonic/gin"
)

// 写作页。正文是纯 Markdown（无预览），只提交 md_content，
// 由后端 goldmark 渲染出 content。
//
// 图片上传分两层：
//   - 有脚本（write.js，纯 ES5）：选完文件立即走 XHR 传到 /lite/upload，
//     封面填进封面框、正文图插到光标处，和主站体验一致；
//   - 没脚本或脚本失败：文件框就在文章表单里（multipart），点「发表 / 保存」时
//     先存图片（封面替换 URL、正文图追加到末尾），再保存文章。

// maxBodyImages 一次最多上传几张正文图（每张上限 10 MiB，见 fileController）
const maxBodyImages = 5

// maxWriteBody 整个写作表单的大小上限：正文图 + 封面各 10 MiB，再留 2 MiB 给文字字段。
// 单张大小在 fileController 里卡，这里防的是一次塞进超大请求体。
const maxWriteBody = int64(maxBodyImages+1)*10240000 + 2<<20

// writeView 写作页公共的那几项
func writeView(c *gin.Context) liteview.WriteView {
	return liteview.WriteView{
		BaseView:   newBaseView(c),
		CanUpload:  hasPerm(c, permission.Perm_UploadFile),
		MaxUploads: maxBodyImages,
	}
}

// handleUploads 保存表单里选中的封面与正文图，结果写回 view。
// 返回值是给用户看的提示（上传了几张）；出错时返回 error，view 不变。
func handleUploads(c *gin.Context, view *liteview.WriteView) (string, error) {
	form := c.Request.MultipartForm
	if form == nil {
		// 不是 multipart（旧页面缓存的普通表单）就当作没有上传
		return "", nil
	}
	cover := nonEmptyFiles(form.File["cover_file"])
	images := nonEmptyFiles(form.File["images"])
	if len(cover) == 0 && len(images) == 0 {
		return "", nil
	}
	if !hasPerm(c, permission.Perm_UploadFile) {
		return "", exception.UsrNotPermitted
	}
	if len(images) > maxBodyImages {
		return "", exception.NewException(exception.ApiFileTooLarge.Code,
			"一次最多上传 "+strconv.Itoa(maxBodyImages)+" 张正文图")
	}

	var coverURL string
	if len(cover) > 0 {
		u, err := fileController.SaveImageFile(cover[0])
		if err != nil {
			return "", err
		}
		coverURL = u
	}
	urls := make([]string, 0, len(images))
	for _, fh := range images {
		u, err := fileController.SaveImageFile(fh)
		if err != nil {
			return "", err
		}
		urls = append(urls, u)
	}

	// 全部成功才写回，避免一半成功一半失败时表单状态不一致
	var parts []string
	if coverURL != "" {
		view.Cover = coverURL
		parts = append(parts, "封面已更新")
	}
	if len(urls) > 0 {
		view.MdBody = liteview.AppendImageMarkdown(view.MdBody, urls)
		parts = append(parts, strconv.Itoa(len(urls))+" 张图片已追加到正文末尾")
	}
	return strings.Join(parts, "，"), nil
}

// nonEmptyFiles 过滤掉空文件：没选文件的 <input type="file"> 有的浏览器也会提交一个空项
func nonEmptyFiles(fs []*multipart.FileHeader) []*multipart.FileHeader {
	out := fs[:0:0]
	for _, f := range fs {
		if f != nil && f.Size > 0 && f.Filename != "" {
			out = append(out, f)
		}
	}
	return out
}

// uploadBody 单张上传的请求体上限：图片 10 MiB，再留 1 MiB 给 multipart 的边界与字段
const uploadBody = int64(10240000 + 1<<20)

// uploadParseErrMsg 上传请求体读不出来时给脚本的文案
func uploadParseErrMsg(err error) string {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return exception.ApiFileTooLarge.Msg
	}
	return exception.ApiNoFormFile.Msg
}

// UploadImage POST /lite/upload：写作页脚本用的图片上传，返回 JSON。
//
// 主站的 /api/upload/image 只认 Authorization 头，/lite 是 cookie 登录，所以单开一个。
// CSRF 由 Guard 校验（脚本用 X-CSRF-Token 头带 token）。
// X-Requested-With 只是确认调用方是 write.js，不承担安全职责。
func UploadImage(c *gin.Context) {
	if c.GetHeader("X-Requested-With") != "XMLHttpRequest" {
		uploadJSON(c, http.StatusBadRequest, exception.ApiParamError.Msg, nil)
		return
	}
	accountID, _ := c.Get("AccountID")
	if id, _ := accountID.(uint64); id == 0 {
		uploadJSON(c, http.StatusUnauthorized, "登录已失效，请重新登录", nil)
		return
	}
	if !hasPerm(c, permission.Perm_PostPost) || !hasPerm(c, permission.Perm_UploadFile) {
		uploadJSON(c, http.StatusForbidden, exception.UsrNotPermitted.Msg, nil)
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, uploadBody)
	fh, err := c.FormFile("file")
	if err != nil {
		uploadJSON(c, http.StatusOK, uploadParseErrMsg(err), nil)
		return
	}
	url, err := fileController.SaveImageFile(fh)
	if err != nil {
		uploadJSON(c, http.StatusOK, errMsg(err), nil)
		return
	}
	uploadJSON(c, http.StatusOK, "", gin.H{"url": url, "thumb_url": liteview.ThumbURL(url)})
}

// uploadJSON 统一的返回格式：{ ok, msg, data }。
// 失败也尽量回 200，老设备的 XHR 拿非 2xx 时 responseText 不一定可靠。
func uploadJSON(c *gin.Context, status int, msg string, data gin.H) {
	c.JSON(status, gin.H{"ok": data != nil, "msg": msg, "data": data})
}

// requireWritePerm 取账号并校验发帖权限，未登录跳登录页。
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
	liteview.Render(c, http.StatusOK, "write", writeView(c))
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

	view := writeView(c)
	view.IsEdit = true
	view.ID = post.ID
	view.Title = post.Title
	view.Subtitle = post.Subtitle
	view.Summary = post.Summary
	view.Cover = post.CoverImage
	view.Category = liteview.JoinList(liteview.ParseJSONList(post.Category))
	view.Tags = liteview.JoinList(liteview.ParseJSONList(post.Tags))
	view.MdBody = post.MdContent
	view.IsDeep = post.IsDeep
	view.Hidden = post.Hidden
	view.NoStats = post.NoStats
	renderWrite(c, view)
}

// writeViewFromForm 用提交上来的字段回填写作页（表单须已解析）
func writeViewFromForm(c *gin.Context) liteview.WriteView {
	view := writeView(c)
	if idStr := strings.TrimSpace(c.Param("id")); idStr != "" {
		view.IsEdit = true
		view.ID, _ = strconv.ParseUint(idStr, 10, 64)
	}
	view.Title = strings.TrimSpace(c.PostForm("title"))
	view.Subtitle = strings.TrimSpace(c.PostForm("subtitle"))
	view.Summary = strings.TrimSpace(c.PostForm("summary"))
	view.Cover = strings.TrimSpace(c.PostForm("cover_image"))
	view.Category = strings.TrimSpace(c.PostForm("category"))
	view.Tags = strings.TrimSpace(c.PostForm("tags"))
	view.MdBody = c.PostForm("md_content")
	view.IsDeep = c.PostForm("is_deep") != ""
	view.Hidden = c.PostForm("hidden") != ""
	view.NoStats = c.PostForm("no_stats") != ""
	return view
}

// renderWriteParseError 表单没能读出来（超限或格式不对），只能给一张空表单
func renderWriteParseError(c *gin.Context, err error) {
	view := writeView(c)
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		view.Error = exception.ApiFileTooLarge.Msg + "（一次提交的图片合计不能超过 60 MB）"
	} else {
		view.Error = exception.ApiParamError.Msg
	}
	if idStr := strings.TrimSpace(c.Param("id")); idStr != "" {
		view.IsEdit = true
		view.ID, _ = strconv.ParseUint(idStr, 10, 64)
	}
	renderWrite(c, view)
}

// writeParamError 校验写作表单的必填项，返回给用户看的错误文案；空串表示校验通过。
func writeParamError(view *liteview.WriteView) string {
	if view.Title == "" {
		return exception.ApiArticleTitleRequired.Msg
	}
	if strings.TrimSpace(view.MdBody) == "" {
		return exception.ApiArticleContentRequired.Msg
	}
	return ""
}

// renderWrite 补上封面预览后渲染写作页
func renderWrite(c *gin.Context, view liteview.WriteView) {
	view.CoverPreview = liteview.ThumbURL(view.Cover)
	liteview.Render(c, http.StatusOK, "write", view)
}

// WriteSubmit POST /lite/write 与 POST /lite/write/:id
//
// 主站用 POST(创建) / PUT(更新) 两个 JSON 接口；表单只能 GET/POST，
// 所以这里按路径里有没有 :id 分派，字段与校验规则照搬。
func WriteSubmit(c *gin.Context) {
	if _, ok := requireWritePerm(c); !ok {
		return
	}
	// Guard 校验 CSRF 时可能已经解析过（那时已限好大小），这里的解析就是空操作
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxWriteBody)
	// 先显式解析：超限时 PostForm 会静默返回空值，用户只会看到「参数错误」。
	// 不是 multipart（旧页面缓存的普通表单）不算错。
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		renderWriteParseError(c, err)
		return
	}

	idStr := strings.TrimSpace(c.Param("id"))
	isEdit := idStr != ""
	if isEdit {
		if _, err := strconv.ParseUint(idStr, 10, 64); err != nil {
			renderError(c, http.StatusNotFound)
			return
		}
	}
	view := writeViewFromForm(c)

	notice, err := handleUploads(c, &view)
	if err != nil {
		view.Error = errMsg(err)
		renderWrite(c, view)
		return
	}

	// 校验与后端接口一致：标题非空，且正文非空
	// （主站允许 content / md_content 二选一，这里只有 Markdown 一种输入）
	if msg := writeParamError(&view); msg != "" {
		view.Error = msg
		if notice != "" {
			// 图片已经存好、地址已经填回表单，提示用户不用重新选
			view.Notice = notice
		}
		renderWrite(c, view)
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
			renderWrite(c, view)
			return
		}
		postID = post.ID
	} else {
		post, err := articleService.CreatePost(fields)
		if err != nil {
			view.Error = errMsg(err)
			renderWrite(c, view)
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
	// 一次性提示：删除提交 CSRF 校验失败时会带回来
	msg, _ := utils.TakeFlash(c)
	liteview.Render(c, http.StatusOK, "confirm-delete", liteview.ConfirmDeleteView{
		BaseView: newBaseView(c),
		ID:       post.ID,
		Title:    post.Title,
		Error:    msg,
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
