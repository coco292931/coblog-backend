# coblog-backend

**coco 的避风港 - 后端服务**

一个基于 **Go + Gin + GORM** 的博客后端。当前代码已覆盖账户注册与激活、邮箱验证码、文章 CRUD、Markdown 渲染预览、图片上传压缩、RSS 订阅和站点统计等核心能力。

---

## 目录

- [技术栈](#技术栈)
- [已实现能力](#已实现能力)
- [快速开始](#快速开始)
- [配置说明](#配置说明)
- [API 概览](#api-概览)
- [项目结构](#项目结构)
- [Docker 部署](#docker-部署)

---

## 技术栈

### 核心框架

- **Go 1.25.1**
- **Gin**：HTTP 路由与中间件
- **GORM**：MySQL 数据访问
- **Viper**：配置读取与热更新

### 关键依赖

- **gin-contrib/cors**：跨域配置
- **bits-and-blooms/bitset**：权限位图
- **yuin/goldmark**：Markdown → HTML 渲染
- **disintegration/imaging**：图片压缩与缩放

> RSS 生成使用标准库 `encoding/xml` 手写 RSS 2.0，不再依赖第三方 feed 库。

---

## 已实现能力

### 1. 自定义二进制 Token 认证

- 使用紧凑二进制结构而不是标准 JWT 载荷
- Token 中包含：`用户 ID`、`权限组 ID`、`过期时间`、签名
- 服务端校验无需查库，适合高频接口鉴权
- `Auth` 与 `LooseAuth` 两套中间件分别用于强登录接口和公开接口

### 2. 细粒度权限系统

- 基于 bitset 的权限判断
- 路由层直接声明权限要求
- 当前文章发布、文件上传、资料读取等接口均已接入权限校验

### 3. 邮箱验证码与账户激活流程

当前后端已支持以下邮件能力：

- **注册验证码**：`POST /api/auth/code/send`，`purpose=register`
- **找回密码验证码**：`POST /api/auth/code/send`，`purpose=reset`
- **邮箱验证码登录**：`POST /api/auth/login/email`，`purpose=login`
- **激活邮件**：注册后签发一次性激活令牌并写入 Redis，未激活登录时会重新发送
- **密码找回**：`POST /api/auth/pwd/reset`

邮件发送基于 `smtp` 配置。**验证码与激活令牌均存放于 Redis**，由 TTL 自动过期，
并借助 Lua 脚本保证「比对 + 删除」原子完成：

- 验证码：10 分钟有效，一次性使用；校验失败不消耗记录（防止暴力枚举打掉合法用户的验证码）
- 重发冷却：60 秒，通过 `SET NX EX` 实现，天然防止并发重复下发；发送失败会释放标记以便立即重试
- 激活令牌：24 小时有效，取出即删除；链接中只有不可猜的随机串，不含任何可篡改信息

> Redis 不可用时这些功能会**明确报错**而非静默降级：激活/验证码属安全敏感流程，
> 宁可失败也不应绕开校验。

### 4. Markdown 优先的文章工作流

- `POST /api/articles`：创建文章
- `PUT /api/articles/:id`：更新文章
- `DELETE /api/articles/:id`：删除文章
- `GET /api/articles/:id/edit`：获取原始文章内容用于编辑回填
- `POST /api/markdown/render`：Markdown 实时预览

文章保存规则：

- 传入 `md_content` 时，以 Markdown 为单一信源
- 后端使用 `goldmark` 渲染 HTML 并保存到 `content`
- 字数统计优先基于 Markdown 原文计算

### 5. 文章列表搜索与筛选

`GET /api/articles` 当前支持：

- `page` / `pageSize`：分页
- `q`：标题、摘要、正文、分类、标签关键词搜索
- `category`：分类筛选
- `tag`：标签筛选

列表接口会根据登录态区分 `def` / `deep` 文章可见范围。

### 6. 图片上传与自动压缩

图片上传接口为 `POST /api/upload/image`，已具备：

- 文件大小限制：**10 MiB**
- 后缀白名单：`.jpg`、`.jpeg`、`.png`、`.gif`、`.webp`
- 超过阈值时自动生成压缩图
- 返回优先使用压缩图 URL，同时保留原图 URL

相关压缩参数通过 `fileobject` 配置控制。

### 7. RSS 订阅

`GET /api/rss` 输出标准 **RSS 2.0**，已支持：

- 通过 `token` 控制是否输出深度内容
- 通过 `category` / `tag` 过滤文章
- 使用站点配置自动填充 RSS 元信息
- 条目包含 `guid` / `dc:creator` / `category`（自动解析分类与标签）/ `content:encoded` 全文
- 有封面图时输出 `enclosure` 与 `media:thumbnail`
- 被编辑过的文章额外输出 `atom:updated`（`pubDate` 始终为创建时间，避免旧文章冒泡）

`atom:link rel="self"` 由 `fileobject.public_base_url` 拼接，并保留当前请求的查询串
（`token` / `category` / `tag`），确保自引用地址返回与当前内容一致的 feed；
该配置留空时不输出 `atom:link`。

### 8. 站点信息接口

`GET /api/site/info` 用于前端页脚展示：

- 总文章数
- 总字数
- 阅读时长估算
- 站点开放时间 / 运行时长

### 9. CORS 与开放域名策略

当前代码默认允许：

- `localhost`
- `127.0.0.1`
- `192.168.*.*`
- `coco-29.wang` 及子域名

适合本地联调、局域网调试与线上域名访问。

---

## 快速开始

### 环境要求

- Go 1.25.1+
- MySQL 5.7 / 8.0+
- 可写的上传目录
- 可选：可用的 SMTP 服务

### 1. 克隆项目

```bash
git clone https://github.com/coco292931/coblog-backend
cd coblog-backend
```

### 2. 准备数据库

```sql
CREATE DATABASE coblog CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

> 当前 `configs/database/database.go` 中默认跳过自动迁移，实际使用前需要先准备好表结构和权限数据。

### 3. 复制配置文件

```bash
copy .\configs\configs\appConfigs_example.yaml .\configs\configs\appConfigs.yaml
```

### 4. 修改配置

至少补齐：

- 数据库连接
- `webtoken_sigkey`
- `fileobject.dir`
- `site.base_url`
- `smtp.*`（若要启用注册/激活/找回密码）

### 5. 安装依赖并启动

```bash
go mod download
go run main.go
```

默认监听地址：`http://localhost:8080`

---

## 配置说明

配置文件路径：`configs/configs/appConfigs.yaml`

示例：

```yaml
database:
  host: localhost
  port: 3306
  username: root
  password: your_password
  dbname: coblog

webtoken_sigkey: your_base64_48_bytes_key

account:
  valid_secs: 2592000

fileobject:
  dir: D:/uploads
  compress_threshold: 524288
  compress_max_width: 1920
  compress_quality: 80
  public_base_url: https://api.coco-29.wang

site:
  base_url: https://coco-29.wang
  title: coco的避风港
  description: coco的个人博客
  author: coco
  email: coco@coco-29.wang
  rss_max_items: 30

smtp:
  host: smtp.gmail.com
  port: 465
  username: your@gmail.com
  password: your_auth_code
  from: your@gmail.com
  from_name: coco的避风港

redis:
  host: 127.0.0.1   # 留空则激活链接与验证码功能不可用
  port: 6379
  password:
  db: 0
```

### 配置项说明

- `webtoken_sigkey`：48 字节随机密钥的 Base64 文本
- `fileobject.dir`：上传文件落盘目录，需事先存在且进程可写
- `fileobject.compress_*`：控制图片压缩阈值、宽度与质量
- `fileobject.public_base_url`：**对外访问的后端基础地址**，用于拼接图片/封面的绝对 URL 以及 RSS 的 `atom:link`；留空则图片返回相对路径、RSS 不输出 `atom:link`
- `site.base_url`：**前端**站点地址，用于拼接文章链接
- `smtp`：注册验证码、激活邮件、找回密码依赖该配置
- `redis`：**账户激活令牌与邮件验证码存于此**。留空则这两个功能直接报错（不静默降级）

---

## API 概览

### 认证相关

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/auth/login/combo` | 邮箱 + 密码登录 |
| `POST` | `/api/auth/login/email` | 邮箱验证码登录 |
| `POST` | `/api/auth/register` | 邮箱验证码注册 |
| `POST` | `/api/auth/code/send` | 发送验证码，`purpose=register/reset/login` |
| `POST` | `/api/auth/pwd/reset` | 通过邮箱验证码重置密码 |
| `GET` | `/api/auth/activate` | 激活账户 |

### 用户相关

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/user/info/` | 获取个人资料 |
| `PUT` | `/api/user/info/` | 更新个人资料 |
| `PUT` | `/api/user/pwd/` | 修改密码 |
| `PUT` | `/api/user/rst-rss/` | 重置 RSS Token |

### 文章相关

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/articles` | 列表 / 搜索 / 分类标签筛选 |
| `GET` | `/api/articles/:id` | 获取文章详情 |
| `POST` | `/api/articles` | 创建文章 |
| `PUT` | `/api/articles/:id` | 更新文章 |
| `DELETE` | `/api/articles/:id` | 删除文章 |
| `GET` | `/api/articles/:id/edit` | 获取编辑回填数据 |
| `POST` | `/api/markdown/render` | Markdown 预览渲染 |

### 文件与站点

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/upload/image` | 上传图片 |
| `GET` | `/api/site/info` | 获取站点统计信息 |
| `GET` | `/api/rss` | 生成 RSS，支持 `token/category/tag` |

### 管理接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/admin/users/` | 获取用户列表 |

---

## 项目结构

```text
coblog-backend/
├── main.go
├── go.mod
├── Dockerfile
├── common/
│   ├── basics/
│   ├── exception/
│   ├── permission/
│   └── webtoken/
├── configs/
│   ├── configReader/
│   ├── configs/
│   ├── database/
│   └── router/
├── controllers/
│   ├── accountControllers/
│   ├── articlesControllers/
│   ├── fileController/
│   ├── loginControllers/
│   ├── markdownController/
│   ├── registerControllers/
│   ├── rssController/
│   └── siteInfoController/
├── middlewares/
├── models/
├── services/
│   ├── articleService/
│   ├── fileService/
│   ├── mailService/
│   ├── markdownService/
│   ├── rssService/
│   ├── siteInfoService/
│   └── userService/
├── dao/
├── utils/
└── docs/
```

---

## Docker 部署

当前 `Dockerfile` 为多阶段构建：

- 构建阶段：`golang:1.25`
- 运行阶段：`scratch`

### 构建镜像

```bash
docker build -t coblog-backend .
```

### 运行示例

```bash
docker run -d \
  -p 8080:8080 \
  -v /host/appConfigs.yaml:/configs/configs/appConfigs.yaml:ro \
  -v /host/uploads:/uploads \
  --name coblog-backend \
  coblog-backend
```

同时需要保证配置中的：

- `fileobject.dir=/uploads`
- 数据库地址对容器可达

---

## 许可证

详见 [LICENSE](LICENSE)

---

**Built with ❤️ by coco & koko**