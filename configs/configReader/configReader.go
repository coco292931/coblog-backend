package configreader

import (
	"bytes"
	_ "embed"
	"log"
	"sync"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

//#####CONST#####

const modelLogName = "CfgReader"
const appConfigPath = "./configs/configs"
const appConfigName = "appConfigs"

// TEST: 内嵌的配置文件
//
var embedAppCfg []byte
var testenv = true

//#####PUBLIC#####

// 定义配置内容结构体，使包外代码不再依赖配置文件的编写
// 注意：此处结构体开放给外部写死不可改变，请通过改变别名tag来对应实际配置的键名
type DatabaseCfg struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

type FileObjectCfg struct {
	Dir              string `mapstructure:"dir"`
	LargeFileSize    int    `mapstructure:"large_file_size"`
	CompressThreshold int64 `mapstructure:"compress_threshold"` // 超过此字节数时压缩，0 表示始终压缩
	CompressMaxWidth  int   `mapstructure:"compress_max_width"`  // 压缩后最大宽度（像素），0 表示不限制宽度
	CompressQuality   int   `mapstructure:"compress_quality"`    // JPEG 压缩质量 1-100，0 使用默认值 80
	PublicBaseURL     string `mapstructure:"public_base_url"`    // 图片对外访问的基础地址，如 https://api.coco-29.wang，用于拼接绝对 URL（RSS 等跨域场景必需）
}

type AccountCfg struct {
	ValidSecs uint64 `mapstructure:"valid_secs"`
}

type SiteCfg struct {
	BaseURL     string `mapstructure:"base_url"`    // 站点前端地址，用于拼接文章链接，如 https://coco-29.wang
	Title       string `mapstructure:"title"`       // 站点标题
	Description string `mapstructure:"description"` // 站点描述
	Author      string `mapstructure:"author"`      // 作者名
	Email       string `mapstructure:"email"`       // 作者邮箱
	RSSMaxItems int    `mapstructure:"rss_max_items"` // RSS 最多输出条数，0 表示使用默认值
}

// SMTPCfg 邮件发送配置，用于注册/找回密码等验证码邮件
type SMTPCfg struct {
	Host          string `mapstructure:"host"`            // SMTP 服务器地址，如 smtp.qq.com
	Port          int    `mapstructure:"port"`            // SMTP 端口，常见 465(SSL) / 587(STARTTLS) / 25
	Username      string `mapstructure:"username"`        // 登录用户名（通常是发件邮箱）
	Password      string `mapstructure:"password"`        // 授权码 / 密码
	From          string `mapstructure:"from"`            // 发件人地址，留空时使用 username
	FromName      string `mapstructure:"from_name"`       // 发件人显示名称
	TLSSkipVerify bool   `mapstructure:"tls_skip_verify"` // 跳过证书验证（服务器缺少CA证书时使用）
}

type InternalAppCfg struct {
	Database       DatabaseCfg   `mapstructure:"database"`
	FileObject     FileObjectCfg `mapstructure:"fileobject"`
	WebtokenSigkey string        `mapstructure:"webtoken_sigkey"`
	Account        AccountCfg    `mapstructure:"account"`
	Site           SiteCfg       `mapstructure:"site"`
	SMTP           SMTPCfg       `mapstructure:"smtp"`
}

// Get 并发安全返回最新配置，这是configReader的唯一对外接口
var initOnce sync.Once

func GetConfig() InternalAppCfg {
	initOnce.Do(initConfigReader) //首次调用初始化（延迟初始化）
	return atomicCfg.Load().(InternalAppCfg)
}

//#####PRIVATE#####

// 存放配置的原子容器，局部变量
var atomicCfg atomic.Value

// 局部配置读写器，使用 viper
var configReader = viper.New()

// 初始化配置读写器

func initConfigReader() {

	// 载入内嵌配置文件(仅供测试环境)
	if embedAppCfg != nil && testenv {
		testenv_readinConfigOnce()
		return
	}

	log.Printf("[INFO][%v] 载入程序配置", modelLogName)
	configReader.AddConfigPath(appConfigPath) //搜索目录
	configReader.SetConfigName(appConfigName) //配置文件名称
	configReader.SetConfigType("yaml")
	//首次读配置文件
	rcfg_err := configReader.ReadInConfig()

	if rcfg_err != nil {
		log.Fatalf("[FATAL][%v] 无法读取配置文件 错误：%v", modelLogName, rcfg_err)
	}
	if err := updateConfig(configReader); err != nil {
		log.Fatalf("[FATAL][%v] 首次解析配置失败 错误: %v", modelLogName, err)
	}
	//实现配置文件热加载
	configReader.WatchConfig()
	configReader.OnConfigChange(hotLoadCfg)
}

func hotLoadCfg(e fsnotify.Event) {
	log.Printf("[WARN][%v] 配置文件变动，开始热加载: %s\n", modelLogName, e.Name)
	if err := updateConfig(configReader); err != nil {
		log.Printf("[ERROR][%v] 热加载失败，配置未更新: %v\n", modelLogName, err)
	}
}

func updateConfig(viper *viper.Viper) error {
	var icfg InternalAppCfg
	if err := viper.Unmarshal(&icfg); err != nil {
		return err
	}
	atomicCfg.Store(icfg)
	log.Printf("[INFO][%v] 程序配置已更新: %+v\n", modelLogName, &icfg)
	return nil
}

func testenv_readinConfigOnce() {
	log.Printf("[WARN][%v] 配置文件读写器处于测试环境, 热加载已禁用", modelLogName)
	v := viper.New()
	v.SetConfigType("yaml")
	v.ReadConfig(bytes.NewReader(embedAppCfg))
	updateConfig(v)
}
