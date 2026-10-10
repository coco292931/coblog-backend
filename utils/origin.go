package utils

import (
	"net"
	"net/url"
	"strings"
)

// siteDomain 生产域名，自身及其所有子域名都放行。
const siteDomain = "coco-29.wang"

// IsAllowedOrigin CORS 来源白名单。
//
// 先解析再比 hostname，不做字符串前缀匹配：按前缀比的话，
// http://localhost.evil.com、http://192.168.evil.com 这类攻击者可注册的域名也能通过。
//
// 放行范围：
//   - 开发：localhost、回环地址、局域网 192.168.0.0/16，任意端口
//   - 生产：coco-29.wang 及其所有子域名
func IsAllowedOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	// 合法的 Origin 头只有 scheme://host[:port]
	if u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}

	host := strings.ToLower(u.Hostname())
	if host == "localhost" || host == siteDomain || strings.HasSuffix(host, "."+siteDomain) {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	v4 := ip.To4()
	return v4 != nil && v4[0] == 192 && v4[1] == 168
}
