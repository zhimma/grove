package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/zhimma/grove/app/worker/internal/server"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/pkg/logger"
)

func main() {
	configFile := flag.String("c", "", "config file path")
	flag.Parse()

	cfg, err := config.LoadWithOptions(config.LoadOptions{
		Service:    "worker",
		ConfigFile: *configFile,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	app, cleanup, err := server.NewServer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建工作进程失败: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	if err := app.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "启动工作进程失败: %v\n", err)
		os.Exit(1)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	if err := app.Stop(context.Background()); err != nil {
		logger.Error().Err(err).Msg("工作进程关闭失败")
	}
}