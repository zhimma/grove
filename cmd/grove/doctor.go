package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zhimma/grove/internal/config"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "检查当前配置与组件启用状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadWithOptions(config.LoadOptions{
				ConfigFile: configFile,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			lines := []string{
				fmt.Sprintf("应用名称: %s", cfg.App.Name),
				fmt.Sprintf("运行环境: %s", cfg.App.Env),
				fmt.Sprintf("API 端口: %s", cfg.Port),
				fmt.Sprintf("Console 端口: %s", cfg.ConsolePort),
				fmt.Sprintf("默认数据库: %s (%s)", statusText(cfg.Databases.Default.Enabled), cfg.Databases.Default.Driver),
				fmt.Sprintf("Redis: %s", statusText(cfg.Redis.Enabled)),
				fmt.Sprintf("任务队列: %s", statusText(cfg.Job.Enabled)),
				fmt.Sprintf("API 权限控制: %s", statusText(cfg.Casbin.Enforcers["api"].Enabled)),
				fmt.Sprintf("Console 权限控制: %s", statusText(cfg.Casbin.Enforcers["console"].Enabled)),
				fmt.Sprintf("默认存储磁盘: %s", cfg.Storage.Default),
				fmt.Sprintf("文档服务: %s", statusText(cfg.Docs.Enabled)),
			}
			for _, line := range lines {
				if err := writeLine(out, line); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func statusText(enabled bool) string {
	if enabled {
		return "已启用"
	}
	return "未启用"
}
