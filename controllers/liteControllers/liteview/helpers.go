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

// ────────────────── 以下均为主站已有展示项的等价实现 ──────────────────
//
// 这些是照着主站前端的算法一比一搬过来的（Footer.vue / article/index.vue）。
// 改动前请先确认主站那边对应的实现也被改了 —— /lite 的原则是「只搬运、不发明」。

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

// FormatSiteReadingTime 站点总字数的阅读时长，对应主站 Footer 的 readingTime：
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
