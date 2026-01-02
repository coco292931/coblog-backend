# coblog-backend

**coco的避风港 - 后端系统**

一个基于 Go + Gin + GORM 构建的高性能个人博客后端系统，采用分层架构设计，支持JWT认证、细粒度权限控制、文件上传、RSS订阅等完整博客功能。

---

## 📋 目录

- [技术栈](#技术栈)
- [核心特性](#核心特性)
- [系统架构](#系统架架构)
- [数据库设计](#数据库设计)
- [认证与权限机制](#认证与权限机制)
- [快速开始](#快速开始)
- [配置说明](#配置说明)
- [API文档](#api文档)
- [项目结构](#项目结构)

---

## 🛠 技术栈

### 核心框架
- **Go 1.25.1** - 高性能编译型语言
- **Gin** - 轻量级高性能Web框架
- **GORM** - Go语言ORM库，支持MySQL
- **Viper** - 配置文件管理

### 关键依赖
- **gin-contrib/cors** - CORS跨域处理
- **bits-and-blooms/bitset** - 高效位图权限验证
- **gorm.io/datatypes** - JSON字段支持

---

## ✨ 核心特性

### 1. 自定义JWT认证系统

#### 轻量级二进制Token设计
传统JWT使用JSON载荷体积较大（200+字节），本系统采用**紧凑二进制格式**压缩至**48字节**（Base64编码后64字符）。

**Token结构：**
```
┌─────────┬────────────┬──────────┬────────┬─────────────┐
│ 用户ID  │ 权限组ID   │ 过期时间 │ 保留位 │  SHA256签名 │
│  8字节  │   4字节    │  8字节   │ 4字节  │   24字节    │
└─────────┴────────────┴──────────┴────────┴─────────────┘
```

**核心流程：**
- **生成**：元数据(24B) + SHA256(元数据+密钥)[前24B] → Base64编码
- **验证**：Base64解码 → 重算签名对比 → 检查过期时间
- **提取**：Little-Endian解析用户ID和权限组ID

**技术特点：**
- **无状态验证**：无需数据库查询，纯计算验证
- **密钥管理**：48字节密钥，`sync.Once`懒加载
- **安全性**：SHA256防篡改 + 时间戳防重放
- **扩展性**：4字节保留位用于未来功能（版本号/刷新机制）

### 2. 细粒度权限控制系统
- **基于Bitset的权限验证**：使用位运算高效判断权限
- **分层权限设计**：
  - 权限ID枚举（0-255）
  - 权限组（数据库存储）
  - 用户-权限组关联
- **权限类别**：
  - 登录认证（001）
  - 个人信息管理（010-013）
  - 文件操作（021-027）
  - 文章操作（031-037）
  - 管理面板（100-106）
- **懒加载机制**：使用`sync.Once`确保权限表只加载一次

### 3. 双模式认证中间件
- **严格认证（Auth）**：
  - 必须携带有效Token
  - 验证失败返回401
  - 注入用户ID和权限组ID到Context
- **松散认证（LooseAuth）**：
  - 允许未登录访问
  - Token有效则注入用户信息
  - Token无效则设置默认值
  - 用于文章列表等公开内容

### 4. 灵活的CORS配置
- **开发环境**：
  - 允许localhost所有端口
  - 允许127.0.0.1所有端口
  - 允许192.168.*.*局域网访问
- **生产环境**：
  - 白名单域名验证（coco-29.wang及子域名）
  - 可配置额外允许的域名
- **安全选项**：
  - 支持Credentials传递
  - 预检请求缓存12小时
  - 严格的请求头和方法控制

### 5. 文章系统
- **双内容格式**：支持富文本和Markdown
- **元数据丰富**：
  - 标题、副标题、摘要
  - 封面图片
  - 分类和标签（JSON数组）
  - 字数统计
  - 浏览量、点赞数
- **深度模式**：支持专属内容

### 6. 统一错误处理
- **中间件式错误处理**：`UnifiedErrorHandler()`
- **业务异常抛出**：使用`c.Error()`传递异常
- **标准化错误响应**：统一的错误码和消息格式

### 7. 文件上传服务
- **图片上传**：支持用户头像、文章配图
- **权限控制**：基于`Perm_UploadFile`权限
- **可配置存储路径**：通过配置文件指定存储目录

### 8. RSS订阅生成
- **动态RSS生成**：根据文章列表生成RSS Feed
- **松散认证**：允许未登录用户订阅
- **个性化订阅**：支持基于RSSToken的个性化订阅

---

## 🏗 系统架构

### 分层架构设计

```
┌─────────────────────────────────────────┐
│          HTTP Request Layer             │
│  (Gin Router + CORS + Middlewares)      │
└───────────────┬─────────────────────────┘
                │
┌───────────────▼─────────────────────────┐
│         Controller Layer                │
│(accountControllers, articlesControllers)|
└───────────────┬─────────────────────────┘
                │
┌───────────────▼─────────────────────────┐
│          Service Layer                  │
│   (articleService, userService)         │
└───────────────┬─────────────────────────┘
                │
┌───────────────▼─────────────────────────┐
│           DAO Layer                     │
│     (Database Access Object)            │
└───────────────┬─────────────────────────┘
                │
┌───────────────▼─────────────────────────┐
│          Model Layer                    │
│    (GORM Models + Database)             │
└─────────────────────────────────────────┘
```

### 请求处理流程

```
Request
  │
  ├─> CORS验证
  │
  ├─> UnifiedErrorHandler中间件
  │
  ├─> Auth/LooseAuth中间件（JWT验证）
  │
  ├─> NeedPerm中间件（权限验证）
  │
  ├─> Controller（业务处理）
  │     │
  │     ├─> Service（业务逻辑）
  │     │     │
  │     │     └─> DAO（数据访问）
  │     │           │
  │     │           └─> Model（数据库）
  │     │
  │     └─> JSON Response / Error
  │
  └─> Response
```

---

## 🗄 数据库设计

### 核心数据表

#### AccountInfo（用户表）
```go
- ID (uint64, PK)              // 用户唯一标识
- Email (string, Index)        // 登录邮箱
- PasswordHash (string)        // bcrypt密码哈希
- UserName (string, Index)     // 用户名
- PermGroupID (uint32, Index)  // 权限组ID
- RSSToken (string)            // RSS订阅令牌
- AvatarFile (string)          // 头像文件名
- Sex / SexInfo (string)       // 性别信息
- Deepable (bool)              // 是否可启用深度模式
- IsDeep (bool)                // 是否已启用深度模式
- Behaviors (text)             // 用户偏好标签（JSON数组）
- Likes (text)                 // 点赞文章列表（JSON数组）
- CreatedAt / UpdatedAt        // 时间戳
- DeletedAt (soft delete)      // 软删除
```

#### Post（文章表）
```go
- ID (uint64, PK)              // 文章唯一标识
- Title (string)               // 标题
- Subtitle (string)            // 副标题
- Summary (string)             // 摘要（列表显示）
- CoverImage (string)          // 封面图片
- Content (string)             // 富文本内容
- MdContent (string)           // Markdown内容
- Category (string, Index)     // 分类（JSON数组）
- Tags (text)                  // 标签（JSON数组）
- IsDeep (bool, Index)         // 是否深度内容
- Words (uint64)               // 字数统计
- Views (uint64)               // 浏览量
- Likes (uint64)               // 点赞数
- Comments (JSON)              // 评论数据（保留）
- CreatedAt / UpdatedAt        // 时间戳
```

#### PermissionGroup（权限组表）
- 使用Bitset存储权限位图
- 支持高效的权限验证（位运算）

---

## 🔐 认证与权限机制

### JWT Token生成流程

```go
1. 用户登录验证（邮箱+密码 或 邮箱+验证码）
2. 查询数据库获取用户ID和权限组ID
3. 生成Token元数据：
   - 用户ID (8字节)
   - 权限组ID (4字节)
   - 过期时间 (8字节, Unix时间戳)
   - 保留位 (4字节)
4. 使用SHA256计算签名：
   SHA256(元数据 + 48字节密钥) → 取前24字节
5. 拼接元数据+签名 = 48字节
6. Base64编码 → 64字符Token
7. 返回给客户端
```

### Token验证流程

```go
1. 从请求头获取Authorization
2. Base64解码 → 48字节二进制
3. 提取元数据（前24字节）
4. 提取签名（后24字节）
5. 使用相同算法重新计算签名
6. 对比签名是否一致
7. 检查过期时间
8. 验证通过 → 提取用户ID和权限组ID
```

### 权限验证流程

```go
1. 从Context获取权限组ID
2. 懒加载权限组配置（首次从数据库读取）
3. 根据权限组ID获取权限Bitset
4. 使用位运算检查所需权限：
   HasPermission = (UserPermBits & RequiredPermBits) == RequiredPermBits
5. 验证通过 → 继续处理请求
6. 验证失败 → 返回403 Forbidden
```

---

## 🚀 快速开始

### 前置要求
- Go 1.25.1 或更高版本
- MySQL 5.7+ 或 8.0+
- Git

### 安装步骤

1. **克隆项目**
```bash
git clone https://github.com/coco292931/coblog-backend
cd ./coblog-backend
```

2. **配置数据库**
```bash
# 创建数据库
mysql -u root -p
CREATE DATABASE coblog CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
#数据库表应该会在首次运行时自动迁移，但权限组表仍需手动配置
```

3. **配置文件**
```bash
# 复制配置示例
cd configs/configs
cp appConfigs_example.yaml appConfigs.yaml

# 编辑配置文件
# 填写数据库连接信息、JWT密钥等
```

4. **安装依赖**
```bash
go mod download
```

5. **运行项目**
```bash
go run main.go
```

服务将在 `http://localhost:8080` 启动

## ⚙️ 配置说明

### appConfigs.yaml 配置文件
可参考 configs\configs\appConfigs_example.yaml
上线需要删除_example后缀

```yaml
# 数据库配置
database:
  host: localhost        # 数据库主机
  port: 3306            # 数据库端口
  username: root        # 数据库用户名
  password: your_password  # 数据库密码
  dbname: coblog        # 数据库名称

# JWT签名密钥（64字符Base64编码）
# 生成方式：openssl rand -base64 48
webtoken_sigkey: YKFAq3akQp5pEEAU8UqnrMGUr9lg4DxZZj6uRkHdcugUAcnxF89HunEkUugD0kIP

# 账户配置
account:
  valid_secs: 2592000  # Token有效期（秒），30天

# 文件存储配置
fileobjects:
  dir: /path/to/uploads  # 文件上传目录
```

### 环境变量（可选）
```bash
export GIN_MODE=release  # 生产模式
export GIN_MODE=debug    # 开发模式（默认）
```

---

## 📡 API文档

### 基础信息
- **Base URL**: `http://localhost:8080/`
- **认证方式**: Bearer Token (JWT)
- **请求头**: 
  - `Content-Type: application/json`
  - `Authorization: <token>`

### 核心端点

#### 认证相关
```
POST   /api/auth/login/combo     # 账号密码登录
POST   /api/auth/login/email     # 邮箱验证码登录
POST   /api/auth/register        # 用户注册
```

#### 用户相关
```
GET    /api/user/info/           # 获取个人信息 [需登录]
PUT    /api/user/info/           # 更新个人信息 [需登录]
PUT    /api/user/pwd/            # 修改密码 [需登录]
PUT    /api/user/rst-rss/        # 重置RSS Token [需登录]
```

#### 文章相关
```
GET    /api/articles             # 获取文章列表 [公开]
GET    /api/articles/:id         # 获取文章详情 [公开/登录]
```

#### 文件相关
```
POST   /api/upload/image         # 上传图片 [需登录+权限]
```

#### 其他
```
GET    /api/site/info            # 获取站点信息 [公开]
GET    /api/rss                  # RSS订阅 [公开]
```

#### 管理员
```
GET    /api/admin/users/         # 获取用户列表 [需管理员权限]
```

---

## 📁 项目结构

```
coblog-backend/
├── main.go                    # 程序入口
├── go.mod                     # Go模块依赖
├── go.sum                     # 依赖校验
├── Dockerfile                 # Docker镜像构建
│
├── common/                    # 公共组件
│   ├── basics/               # 基础工具（类型转换等）
│   ├── exception/            # 异常定义和错误码
│   ├── permission/           # 权限系统核心逻辑
│   │   ├── permission.go    # 权限验证、权限枚举
│   │   └── configs/         # 权限组配置
│   └── webtoken/            # JWT Token生成与验证
│
├── configs/                  # 配置模块
│   ├── configReader/        # Viper配置读取
│   ├── configs/             # 配置文件目录
│   │   ├── appConfigs.yaml        # 实际配置（不提交）
│   │   └── appConfigs_example.yaml # 配置示例
│   ├── database/            # GORM数据库连接
│   └── router/              # Gin路由配置
│       └── router.go        # 路由注册、CORS配置
│
├── controllers/             # 控制器层（处理HTTP请求）
│   ├── accountControllers/  # 账户管理
│   ├── articlesControllers/ # 文章管理
│   ├── fileController/      # 文件上传
│   ├── loginControllers/    # 登录逻辑
│   ├── registerControllers/ # 注册逻辑
│   ├── rssController/       # RSS生成
│   └── siteInfoController/  # 站点信息
│
├── middlewares/             # 中间件
│   ├── auth.go             # JWT认证中间件
│   ├── checkPerm.go        # 权限验证中间件
│   └── errorHandler.go     # 统一错误处理
│
├── models/                 # 数据模型（GORM）
│   ├── account.go         # 用户模型
│   ├── post.go            # 文章模型
│   ├── permission.go      # 权限组模型
│   ├── comments.go        # 评论模型（保留）
│   └── siteInfo.go        # 站点信息模型
│
├── services/              # 业务逻辑层
│   ├── articleService/    # 文章业务逻辑
│   ├── userService/       # 用户业务逻辑
│   ├── fileService/       # 文件处理逻辑
│   └── ssrService/        # 服务端渲染（保留）
│
├── dao/                   # 数据访问层
│   ├── account.go        # 用户数据访问
│   ├── permission.go     # 权限数据访问
│   └── Readme.md
│
├── utils/                # 工具函数
│   └── jsonResponse.go  # JSON响应封装
│
└── docs/                 # 项目文档
    ├── ProjectStructure.md  # 项目结构说明
    ├── ErrorCodes.md        # 错误码列表
    ├── DataFormat.md        # 数据格式规范
    ├── GinContext.md        # Gin上下文使用
    └── 权限列表.txt         # 权限清单
```

---

## 🐳 Docker部署

```bash
# 构建镜像
docker build -t coblog-backend .

# 运行容器
docker run -d \
  -p 8080:8080 \
  -v /path/to/configs:/app/configs \
  -v /path/to/uploads:/app/uploads \
  --name coblog-backend \
  coblog-backend
```

---

## 📝 许可证

详见 [LICENSE](LICENSE) 文件

---

## 🤝 贡献指南

欢迎提交Issue和Pull Request！

---

## 📮 联系方式

- 项目主页: coco@coco-29.wang
- 问题反馈: [GitHub Issues]

---

**Built with ❤️ by coco&koko**
