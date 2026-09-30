package main

import (
	"github.com/spf13/cobra"
)

func newAboutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "about",
		Short: "显示当前框架约定与可用能力",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			for _, line := range []string{
				"Grove 基础框架",
				"- 单仓服务：api / console / worker",
				"- CLI 入口：grove",
				"- 默认日志：pkg/logger + zerolog",
				"- 默认校验：make verify",
				"- make:module 生成 console 模块：迁移、模型、CRUD、接口文档与后台页面",
			} {
				if err := writeLine(out, line); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
