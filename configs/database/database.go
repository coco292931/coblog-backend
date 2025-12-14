package database

import (
	configreader "JHETBackend/configs/configReader"
	"JHETBackend/models"
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/microsoft/go-mssqldb"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

var DataBase *gorm.DB

var initDBOnce sync.Once

func init() { // 立即加载数据库，根据需要可以删除
	initDBOnce.Do(initDatabase)
}

func initDatabase() {

	//TODO: 判断连接成功和报错的逻辑还要再改，研究下自动迁移相关的设置

	// 抄了一点4UOnline-Go的代码 折乙你不会生气吧（

	// 从配置中获取数据库连接所需的参数
	user := configreader.GetConfig().Database.Username // 数据库用户名
	pass := configreader.GetConfig().Database.Password // 数据库密码
	host := configreader.GetConfig().Database.Host     // 数据库主机
	port := configreader.GetConfig().Database.Port     // 数据库端口
	name := configreader.GetConfig().Database.DBName   // 数据库名称

	// 构建 Azure SQL Database 连接字符串（添加连接超时参数，适配 Serverless 数据库唤醒时间）
	connString := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d;database=%s;connection timeout=60;",
		host, user, pass, port, name)

	log.Printf("[INFO][DB] 尝试连接到 Azure SQL Database (Serverless 数据库唤醒可能需要120s，请稍候...)")

	// 创建连接池
	db, err := sql.Open("sqlserver", connString)
	if err != nil {
		log.Panicf("[FATAL] Error creating connection pool: %v", err.Error())
	}

	// 设置连接池参数
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)

	// 测试连接（使用带超时的 context，给 Serverless 数据库足够的唤醒时间）
	ctx, cancel := context.WithTimeout(context.Background(), 130*time.Second)
	defer cancel()

	log.Printf("[INFO][DB] 正在测试数据库连接...")
	err = db.PingContext(ctx)
	if err != nil {
		log.Panicf("[FATAL] Database connection failed: %v", err.Error())
	}

	log.Printf("[INFO][DB] 成功连接到 Azure SQL Database")

	// 使用 GORM 包装 SQL Server 连接
	var dbtmp *gorm.DB
	dbtmp, err = gorm.Open(sqlserver.New(sqlserver.Config{
		Conn: db,
	}), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true, // 关闭外键约束以提升迁移速度
	})
	if err != nil {
		log.Panicf("[FATAL] Open database with GORM failed. Err: %v", err)
	}

	log.Printf("[INFO][DB] 数据库连接成功, 当前数据库: %s", name)
	//连接成功传递到全局指针
	DataBase = dbtmp

	// 自动迁移数据库表结构
	log.Printf("[INFO][DB] 开始自动迁移数据库表结构...")
	err = autoMigrate(dbtmp)
	if err != nil {
		log.Panicf("[FATAL] 数据库表迁移失败: %v", err)
	}
	log.Printf("[INFO][DB] 数据库表迁移完成！")
}

// autoMigrate 自动迁移所有数据表
func autoMigrate(db *gorm.DB) error {
	//迁移一次就行了
	log.Printf("[INFO][DB] 本次跳过迁移")
	return nil
	// 迁移所有模型
	return db.AutoMigrate(
		&models.AccountInfo{},
		// 如果有其他模型，在这里继续添加
	)
}
