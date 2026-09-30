package main

import (
	"github.com/spf13/cobra"
)

var configFile string

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "grove",
		Short:        "框架工具：迁移、数据填充、代码生成与环境信息查看",
		SilenceUsage: true,
		Long: `grove 是当前仓库唯一保留的 CLI 入口。

适合做四类事情：
1. 迁移与 seed
2. 生成 console 后台约定代码
3. 生成强随机密钥
4. 查看当前框架约定与环境信息

make:module 一条命令生成可运行的后台模块：双方言迁移、模型、
分页 CRUD 与测试、接口文档、路由注册，以及后台前端的接口、页面与菜单。`,
	}
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "配置文件路径")

	rootCmd.AddCommand(newAboutCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newMigrateCmd())
	rootCmd.AddCommand(newSeedCmd())
	rootCmd.AddCommand(newRBACCmd())
	rootCmd.AddCommand(newMakeModuleCmd())
	rootCmd.AddCommand(newKeyGenerateCmd())

	return rootCmd
}
