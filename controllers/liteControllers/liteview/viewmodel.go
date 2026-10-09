package liteview

import "html/template"

// /lite 各页面模板的视图模型。展示项只搬运主站已有的，不自己发明。

// BaseView 所有页面共用的部分。没有页面标题字段 —— 主站的 <title>
// 与导航栏品牌都是 index.html 里的固定字符串，跟路由无关。
type BaseView struct {
	CurrentPath string
	// NavKey 当前所在的导航项，供模板加高亮；空串表示无匹配项。
	NavKey   string
	LoggedIn bool // 决定导航栏是否显示「写作」
	Stats    *StatsView
}

// StatsView 页脚统计，对应主站 Footer 里需要计算的三项。
type StatsView struct {
	TotalWords  string // 站点总字数，已按主站的 formatNumber 缩写（如 34.6k）
	ReadingTime string // 阅读时长，已按主站的 readingTime 格式化（如 0:38）
	Uptime      string // 已避风 X天X时X分X秒（服务端渲染那一刻的快照）
}

// ListItemView 对应主站 ArticleTimeline 的一个 .article。
// 无封面时直接不渲染（主站显示占位图）。
type ListItemView struct {
	ID         uint64
	Title      string
	Summary    string // 主站的 article.description
	Cover      string // 已带 ?thumb=1
	Date       string // 与主站 formatDate 一致：2026-01-02
	Categories []string
}

// HomeView 首页。对应主站首页：一句话 + 文章时间线。
type HomeView struct {
	BaseView
	Articles []ListItemView
}

// ListView 文章列表页，对应主站 pages/search/index.vue 的展示项。
type ListView struct {
	BaseView
	Articles []ListItemView

	// 结果统计栏（主站「全部文章：N 篇」/「符合条件的文章：N 篇（…）」）
	Total      int64
	HasFilter  bool
	FilterText string

	// 主站是无限滚动，没有分页控件；这里是 MPA 必需的分页导航
	Page       uint64
	TotalPages uint64
	HasPrev    bool
	HasNext    bool
	PrevURL    string
	NextURL    string

	// 表单回填与筛选
	Keyword  string
	Category string
	Tag      string

	// 排序（主站「最新发布」/「最近修改」，这里用链接实现）
	SortUpdated      bool
	SortPublishedURL string
	SortUpdatedURL   string
}

// ArticleView 文章详情页，对应主站 pages/article/index.vue 的展示项。
// 封面、标题、副标题、创建/修改时间、分类（无分类时显示「未分类」）、
// 阅读时长（分钟）、正文、版权声明、文章统计。
type ArticleView struct {
	BaseView
	Title      string
	Subtitle   string
	Cover      string
	Content    template.HTML // 后端已渲染好的富文本，直接输出（与主站 v-html 行为一致）
	CreateTime string        // 与主站的 formatDateTime 一致：2026/01/02 15:04
	UpdateTime string
	Categories []string
	ReadingMin int // 主站 article 页的 readingTime

	// 版权声明（主站 license-info）。后端 Post 没有作者字段，
	// 所以作者固定用 liteArticleAuthor 的值。
	Author     string
	ArticleURL string

	// 文章统计（主站 info-items）。与主站一样，这三个值是后端返回什么就显示什么。
	Views    uint64
	Likes    uint64
	Comments uint64
}

// AboutView 关于页。/about 的三个地址共用一套模板，用 Tab 切换正文。
type AboutView struct {
	BaseView
	Tab string // "us" | "friends"
}

// LoginView 登录页。字段与主站 regAlogin 页一致。
// 主站是「登录 / 注册」双态页，这里拆成两个页面（MPA 不需要切态）。
type LoginView struct {
	BaseView
	Account  string // 提交失败后回填
	Error    string // 直接显示后端业务错误的原文（与主站弹的 msg 一致）
	Redirect string // 登录成功后的回跳地址（只接受站内相对路径）
}

// WriteView 写作 / 编辑页。正文是纯 Markdown textarea（无预览），
// 分类与标签是逗号分隔的文本框，封面填 URL 或随表单上传。
type WriteView struct {
	BaseView

	IsEdit   bool
	ID       uint64
	Title    string
	Subtitle string
	Summary  string
	Cover    string
	Category string // 逗号分隔（回填与提交都用它）
	Tags     string
	MdBody   string // Markdown 正文
	IsDeep   bool
	Hidden   bool
	NoStats  bool

	CoverPreview string // 封面预览地址（站内图已带 ?thumb=1），空表示不显示预览
	CanUpload    bool   // 有上传图片权限才显示文件选择框
	MaxUploads   int    // 一次最多上传几张正文图

	Error  string
	Notice string
}

// ConfirmDeleteView 删除确认页。主站是在弹窗里输入完整标题确认，
// 这里换成独立页面。
type ConfirmDeleteView struct {
	BaseView
	ID    uint64
	Title string
	Error string
}

// RSSView RSS 订阅页。没有主站那个「复制」按钮（剪贴板需要脚本）。
type RSSView struct {
	BaseView
	URL string
}

// ForgotPasswordView 找回密码页。
// 注册邮箱 / 邮箱验证码 / 新密码 / 确认新密码。
//
// 主站把「获取验证码」做成页面内的一个按钮（需要脚本）；这里是同一个表单里的
// 第二个提交按钮（name=action value=send），行为等价。
type ForgotPasswordView struct {
	BaseView
	Email  string
	Error  string
	Notice string

	PasswordRule string
}

// RegisterView 注册页。字段与主站 regAlogin 的注册态一致：
// 用户名 / 邮箱 / 密码 / 确认密码，外加主站那句「注册成功后请使用邮件中的链接完成账户激活。」
type RegisterView struct {
	BaseView
	Username string
	Email    string
	Error    string // 失败提示
	Notice   string // 成功提示（三种文案与 JSON 注册接口一致）

	PasswordRule string
}

// ActivateView 激活结果页。
type ActivateView struct {
	BaseView
	Success bool
	Title   string
	Message string
}

// MeView 个人中心。展示项与操作照搬主站 pages/me/index.vue：
// 用户名、邮箱、激活状态、深度权限/状态、RSS Token、修改密码、重置 RSS Token、退出登录。
//
// 主站把两个安全操作做成折叠面板（需要脚本），这里直接展开。
type MeView struct {
	BaseView

	// Denied 对应主站的 permDenied 分支：已登录但权限组读不了个人信息
	Denied   bool
	FetchErr string

	Username  string
	Email     string
	Activated bool
	Deepable  bool
	IsDeep    bool
	RSSToken  string

	Message string // 上一次操作的结果
	OK      bool   // 操作是否成功（决定提示框样式）

	PasswordRule string // 新密码强度提示，主站是 input 的 placeholder
}

// ErrorView 错误页。
//
// 文案固定用主站 notFound 页的那三句（见模板），所以这里只需要状态码与当前地址。
type ErrorView struct {
	BaseView
	Code int
}
