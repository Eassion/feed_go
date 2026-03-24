package main

import (
	"enterprise/config"
	"enterprise/internal/handler"
	"enterprise/internal/repository"
	"enterprise/internal/service"
	"enterprise/pkg/cache"
	"enterprise/pkg/database"
	"fmt"

	"github.com/gin-gonic/gin"
)

func main() {
	config.InitConfig()
	db, err := database.NewDB(&config.AppConfig.DatabaseConfig)
	if err != nil {
		panic(err)
	}
	err = database.AutoMigrate(db)
	if err != nil {
		panic(err)
	}
	defer database.CloseDB(db)
	cacheClient, err := cache.NewRedis(&config.AppConfig.RedisConfig)
	if err != nil {
		panic(err)
	}
	defer cacheClient.Close()
	
	// 初始化依赖
	accountRepo := repository.NewAccountRepository(db)
	accountService := service.NewAccountService(accountRepo, cacheClient)
	accountHandler := handler.NewAccountHandler(accountService)
	
	// 初始化路由
	r := gin.Default()
	
	// 注册路由
	r.POST("/api/account/create", accountHandler.CreateAccount)
	r.POST("/api/account/rename", accountHandler.Rename)
	r.POST("/api/account/change-password", accountHandler.ChangePassword)
	r.POST("/api/account/find-by-id", accountHandler.FindByID)
	r.POST("/api/account/find-by-username", accountHandler.FindByUsername)
	r.POST("/api/account/login", accountHandler.Login)
	r.POST("/api/account/logout", accountHandler.Logout)
	
	// 启动服务器
	port := config.AppConfig.ServerConfig.Port
	fmt.Printf("Server starting on port %d...\n", port)
	if err := r.Run(fmt.Sprintf(":%d", port)); err != nil {
		panic(err)
	}
}
