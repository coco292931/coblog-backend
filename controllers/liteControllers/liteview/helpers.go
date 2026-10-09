package liteview

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"coblog-backend/models"
)

// 这些都是纯函数（不碰数据库、不读配置），方便单独测试。

// MaxPage 页码上限，避免构造超大查询。
const MaxPage = 1000000

// UploadURLPrefix 本站上传图片的路径前缀，与 fileController 对外暴露的保持一致。
const UploadURLPrefix = "/static/uploads/"

// ParsePage 解析页码，非法值与越界值一律收敛到合法范围。
func ParsePage(raw string) uint64 {
	n, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil || n == 0 {
		return 1
	}
	if n > MaxPage {
		return MaxPage
	}
	return n
}

// PrevPage 上一页页码，第 1 页时仍是 1（此时按钮是禁用的，链接不会被点到）。
func PrevPage(page uint64) uint64 {
	if page <= 1 {
		return 1
	}
	return page - 1
}

// BuildPageURL 拼分页链接：保留当前的关键词 / 分类 / 标签 / 排序，只换页码。
func BuildPageURL(base string, page uint64, q, category, tag, sort string) string {
	values := url.Values{}
	if q != "" {
		values.Set("q", q)
	}
	if category != "" {
		values.Set("category", category)
	}
	if tag != "" {
		values.Set("tag", tag)
	}
	if sort != "" {
		values.Set("sort", sort)
	}
	values.Set("page", strconv.FormatUint(page, 10))
	return base + "?" + values.Encode()
}

// ParseJSONList 解析库里的 JSON 数组字符串（category / tags）。
// 与前端 parseJsonArray 行为一致：解析失败时按逗号兜底切分，兼容老数据。
func ParseJSONList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err == nil {
		out := make([]string, 0, len(list))
		for _, s := range list {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '，' })
	out := make([]string, 0, len(parts))
	for _, s := range parts {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// FormatDay 日期只到天。零值返回空串，模板据此决定是否输出。
func FormatDay(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// LatestTime 取创建与修改中较晚的一个。与主站首页的时间线口径保持一致。
func LatestTime(p models.Post) time.Time {
	if p.UpdatedAt.After(p.CreatedAt) {
		return p.UpdatedAt
	}
	return p.CreatedAt
}

// ThumbURL 给本站图片地址加上 ?thumb=1，交给后端换成压缩图（没有压缩图时自动回退原图）。
// 幂等：已经带 thumb=1 的地址原样返回。
func ThumbURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.Contains(raw, UploadURLPrefix) {
		return raw
	}
	if strings.Contains(raw, "thumb=1") {
		return raw
	}
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	return raw + sep + "thumb=1"
}

var (
	imgTagRe  = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	srcAttrRe = regexp.MustCompile(`(?is)\ssrc\s*=\s*"([^"]*)"`)
)

// RewriteContentImages 处理正文里的 <img>：
//
//   - 本站图（含 /static/uploads/）→ 统一加 ?thumb=1，省流量；
//   - 以 .webp 结尾的外链 → 降级成文本链接（老 Kindle 不能显示 WebP，
//     留着只会是一个空白框）；
//   - 其余外链原样保留。
//
// 不要在这里改动 img 的其它属性（宽高、alt），模板侧统一限制显示宽度。
func RewriteContentImages(content string) string {
	if content == "" || !strings.Contains(content, "<img") {
		return content
	}
	return imgTagRe.ReplaceAllStringFunc(content, func(tag string) string {
		m := srcAttrRe.FindStringSubmatch(tag)
		if m == nil {
			return tag
		}
		src := m[1]
		switch {
		case strings.Contains(src, UploadURLPrefix):
			return strings.Replace(tag, m[1], ThumbURL(src), 1)
		case IsWebPURL(src):
			return `<a href="` + src + `">[外部图片]</a>`
		default:
			return tag
		}
	})
}

// IsWebPURL 判断外链是否为 WebP（老 Kindle 不支持该格式）。
func IsWebPURL(raw string) bool {
	path := raw
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	return strings.HasSuffix(strings.ToLower(path), ".webp")
}

// FormatCompactNumber 数字缩写，对应主站 Footer 的 formatNumber：
// ≥1e6 → "x.xm"，≥1000 → "x.xk"，其余原样。
// 入参是后端返回的字符串（models.SiteInfo 的计数字段都是 string）。
func FormatCompactNumber(raw string) string {
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		// 解析不了就原样吐回去，不自己编一个值
		return raw
	}
	switch {
	case n >= 1000000:
		return strconv.FormatFloat(float64(n)/1000000, 'f', 1, 64) + "m"
	case n >= 1000:
		return strconv.FormatFloat(float64(n)/1000, 'f', 1, 64) + "k"
	}
	return strconv.FormatInt(n, 10)
}

// FormatSiteReadingTime 站点总字数的阅读时长
// 按每分钟 300 字，输出 "H:MM"（分钟补零、小时不补）。
func FormatSiteReadingTime(rawWords string) string {
	words, err := strconv.ParseInt(strings.TrimSpace(rawWords), 10, 64)
	if err != nil || words <= 0 {
		return "0:00"
	}
	minutes := (words + 299) / 300 // 等价于 Math.ceil(words / 300)
	return strconv.FormatInt(minutes/60, 10) + ":" + fmt.Sprintf("%02d", minutes%60)
}

// ReadingMinutes 单篇文章的阅读时长（分钟），对应主站 article 页：
// 不足 200 字算 1 分钟，否则 ceil(words / 200)。
func ReadingMinutes(words uint64) int {
	if words < 200 {
		return 1
	}
	return int((words + 199) / 200)
}

// FormatDateTime 对应主站 article 页的 formatDateTime：
// zh-CN 的 "2006/01/02 15:04"；早于 2000 年的（含零值）返回空串。
func FormatDateTime(t time.Time) string {
	if t.IsZero() || t.Year() < 2000 {
		return ""
	}
	return t.Format("2006/01/02 15:04")
}

// FormatUptime 对应主站 Footer 的 uptimeDisplay：
// "已避风 X天X时X分X秒"（未开港时为 "本港湾还有 … 开放"）。
//
// ⚠️ 主站是每秒刷新的实时值；这里只能给服务端渲染那一刻的快照 ——
// 老 Kindle 的定时器不可信（100ms 会退化成 ~400ms），不为此在页面上挂脚本。
func FormatUptime(started, now time.Time) string {
	diff := now.Sub(started)
	neg := diff < 0
	if neg {
		diff = -diff
	}

	total := int64(diff.Seconds())
	days := total / 86400
	hours := (total % 86400) / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60

	if neg {
		// 主站是先算出负值再取反；这里 diff 已经取了绝对值，直接用正数
		return fmt.Sprintf("本港湾还有 %d天%d时%d分%d秒 开放", days, hours, minutes, seconds)
	}
	return fmt.Sprintf("已避风 %d天%d时%d分%d秒", days, hours, minutes, seconds)
}

// NavKeyFor 由请求路径推出当前所在的导航项（articles / write / rss / about / me），
// 供模板给对应项加高亮。语义与主站的 router-link-active 一致：详情页归它的上级栏目。
//
// ⚠️ 传入的应当是 URL.Path（不含查询串）—— 否则 /lite/articles?page=2 匹配不上。
func NavKeyFor(path string) string {
	switch {
	case path == "/lite/articles" || strings.HasPrefix(path, "/lite/articles/"):
		return "articles"
	case path == "/lite/write" || strings.HasPrefix(path, "/lite/write/"):
		return "write"
	case path == "/lite/rss":
		return "rss"
	case path == "/lite/about" || strings.HasPrefix(path, "/lite/about/"):
		return "about"
	case path == "/lite/me":
		return "me"
	}
	return ""
}

// SplitList 把用户输入的逗号分隔串拆成去空白的列表。中英文逗号都认。
func SplitList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '，' })
	out := make([]string, 0, len(parts))
	for _, s := range parts {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// JoinList 与 SplitList 互逆，用于把库里的 JSON 数组回填到逗号分隔输入框。
func JoinList(list []string) string { return strings.Join(list, ",") }

// ToJSONList 把用户输入的逗号分隔串转成库里存的 JSON 数组字符串。
// 与主站 buildArticlePayload 的做法一致（trim 后 JSON 化）。
func ToJSONList(raw string) string {
	list := SplitList(raw)
	if len(list) == 0 {
		return ""
	}
	b, err := json.Marshal(list)
	if err != nil {
		return ""
	}
	return string(b)
}

// PasswordRuleText 新密码强度提示。
// 与主站 constants/account.js 的 PASSWORD_RULE_TEXT 保持一致。
const PasswordRuleText = "至少 6 位，建议同时包含字母与数字"

// ValidateNewPassword 校验新密码，返回错误文案；通过时返回空串。
// 逐条对应主站 constants/account.js 的 validateNewPassword，
// 「找回密码」页与「我的」页共用同一套规则。
func ValidateNewPassword(password, confirm string) string {
	if password == "" {
		return "请输入新密码"
	}
	if len([]rune(password)) < 6 {
		return "新密码长度至少需要6位字符"
	}
	if password != confirm {
		return "两次输入的密码不一致，请重新输入"
	}
	return ""
}

// SafeRedirect 只接受站内相对路径作为回跳地址。
//
// 不校验的话，?redirect=https://evil.com 会把这个登录页变成开放重定向的跳板；
// 「//evil.com」这种协议相对写法也要一并挡掉。
func SafeRedirect(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return ""
	}
	if strings.Contains(raw, "\\") {
		return ""
	}
	return raw
}

// FormatFilterText 对应主站 search 页的 activeFilterText：
// 把生效的筛选条件拼成「搜索"x" · 分类"y" · 标签"z"」。
func FormatFilterText(q, category, tag string) string {
	parts := make([]string, 0, 3)
	if q != "" {
		parts = append(parts, "搜索“"+q+"”")
	}
	if category != "" {
		parts = append(parts, "分类“"+category+"”")
	}
	if tag != "" {
		parts = append(parts, "标签“"+tag+"”")
	}
	return strings.Join(parts, " · ")
}
