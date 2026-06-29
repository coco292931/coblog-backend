package mailService

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	configreader "coblog-backend/configs/configReader"
)

// ErrSMTPNotConfigured 表示 SMTP 未配置，发信被跳过
var ErrSMTPNotConfigured = errors.New("SMTP 未配置")

// SendMail 发送一封纯文本/HTML 邮件。
// 自动根据端口选择加密方式：465 走隐式 TLS，其余(587/25)走 STARTTLS。
func SendMail(to, subject, htmlBody string) error {
	cfg := configreader.GetConfig().SMTP
	if cfg.Host == "" || cfg.Username == "" {
		return ErrSMTPNotConfigured
	}

	from := cfg.From
	if from == "" {
		from = cfg.Username
	}

	msg := buildMessage(from, cfg.FromName, to, subject, htmlBody)
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)

	if cfg.Port == 465 {
		return sendImplicitTLS(addr, cfg.Host, auth, from, to, msg)
	}
	return sendSTARTTLS(addr, cfg.Host, auth, from, to, msg)
}

// buildMessage 组装符合 RFC 5322 的邮件报文，正文为 HTML
func buildMessage(from, fromName, to, subject, htmlBody string) []byte {
	fromHeader := from
	if fromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", fromName, from)
	}
	var b strings.Builder
	b.WriteString("From: " + fromHeader + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	return []byte(b.String())
}

// sendImplicitTLS 端口 465：连接即 TLS
func sendImplicitTLS(addr, host string, auth smtp.Auth, from, to string, msg []byte) error {
	tlsConfig := &tls.Config{ServerName: host}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS连接失败: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %w", err)
	}
	defer client.Close()

	return deliver(client, auth, from, to, msg)
}

// sendSTARTTLS 端口 587/25：明文连接后升级为 TLS
func sendSTARTTLS(addr, host string, auth smtp.Auth, from, to string, msg []byte) error {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("创建SMTP客户端失败: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return fmt.Errorf("STARTTLS失败: %w", err)
		}
	}
	return deliver(client, auth, from, to, msg)
}

// deliver 在已建立的 SMTP 会话上完成认证与投递
func deliver(client *smtp.Client, auth smtp.Auth, from, to string, msg []byte) error {
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP认证失败: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("设置收件人失败: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("获取数据写入器失败: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("写入邮件内容失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("关闭数据写入器失败: %w", err)
	}
	return client.Quit()
}
