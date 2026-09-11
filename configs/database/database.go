package database

import (
	configreader "coblog-backend/configs/configReader"
	"coblog-backend/models"
	"fmt"
	"log"
	"sync"

	"gorm.io/driver/mysql"
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

	// 构建数据源名称 (DSN)
	dsn := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v?charset=utf8mb4&parseTime=True&loc=Local",
		user, pass, host, port, name)

	log.Printf("[INFO][DB] 尝试连接到数据库, dsn=%v", dsn)
	var dbtmp *gorm.DB
	var err error
	// 使用 GORM 打开数据库连接
	dbtmp, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true, // 关闭外键约束以提升迁移速度
	})
	if err != nil {
		log.Panicf("[FATAL] Open database failed. Err: %v", err)
	}

	log.Printf("[INFO][DB] 数据库连接成功, 当前数据库: %s", name)
	//连接成功传递到全局指针
	DataBase = dbtmp

	// 自动迁移数据库表结构
	if err := autoMigrate(dbtmp); err != nil {
		log.Panicf("[FATAL] 数据库表迁移失败: %v", err)
	}

	// 回填存量用户的激活状态：历史账户 activation 为空，新增激活校验后会被全部锁死，
	// 这里幂等地将其标记为已激活（仅影响 activation 为空字符串的旧数据）。
	backfillActivation(dbtmp)
}

// backfillActivation 将 activation 为空的存量账户标记为已激活，幂等可重复执行。
func backfillActivation(db *gorm.DB) {
	res := db.Model(&models.AccountInfo{}).
		Where("activation = ? OR activation IS NULL", "").
		Update("activation", "activated")
	if res.Error != nil {
		log.Printf("[WARN][DB] 存量账户激活状态回填失败: %v", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		log.Printf("[INFO][DB] 已回填 %d 个存量账户为已激活", res.RowsAffected)
	}
}

// autoMigrate 自动迁移所有数据表。
// 模型标签已与线上库结构对齐，迁移是幂等的：结构一致时不产生任何 DDL，
// 仅在首次引入新字段（如软删除 deleted_at）时增列与索引。
func autoMigrate(db *gorm.DB) error {
	log.Printf("[INFO][DB] 开始自动迁移数据库表结构...")
	if err := db.AutoMigrate(
		&models.AccountInfo{},
		&models.Post{},
		&models.Comments{},
		&models.SiteInfo{},
		//&models.PermissionGroup{},  //应该按dao里的模型配置
		// 如果有其他模型，在这里继续添加
	); err != nil {
		// 迁移失败不应导致进程退出：服务仍可基于既有表结构运行，
		// 但需明确告警，避免结构不一致被静默忽略。
		log.Printf("[ERROR][DB] 数据库自动迁移失败: %v", err)
		return err
	}
	log.Printf("[INFO][DB] 数据库表迁移完成！")
	return nil
}
