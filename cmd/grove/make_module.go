package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMakeModuleCmd() *cobra.Command {
	var fieldSpec, label string
	cmd := &cobra.Command{
		Use:   "make:module [name]",
		Short: "生成可运行的 console 模块：迁移、模型、CRUD、接口文档与测试",
		Long: `生成一个可以直接运行、并通过 make contracts 的 console 模块。

会生成：
- database/migrations/{postgres,mysql} 同版本迁移
- internal/model 模型
- app/console/internal/service 分页 CRUD 与对应测试
- app/console/internal/handler 请求、响应与路由（含权限名）
- app/console/internal/docs OpenAPI 操作
- 路由注册与 OpenAPI 注册
- 存在后台前端时：api 模块、基于 resource-page 的页面、菜单路由，
  并登记到 console-contract.json

字段写法：name:type[:required]，逗号分隔。
类型：string、text、int、bool、time；required 仅用于 string、text、time。

示例：
  grove make:module Invoice --label 发票 --fields "title:string:required,amount:int,paid:bool,due_at:time"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := generateConsoleModule(args[0], fieldSpec, label)
			if err != nil {
				return err
			}

			fmt.Println("已生成以下文件：")
			for _, path := range paths {
				fmt.Println(path)
			}

			if !hasConsoleFrontend() {
				fmt.Println("未找到后台前端工程，已跳过前端生成。")
			} else if formatted, err := formatFrontend(paths); err != nil {
				fmt.Printf("前端文件已生成但格式化失败，请在 web/admin-vben 运行 pnpm format：%v\n", err)
			} else if !formatted {
				fmt.Println("未安装前端依赖，前端文件未格式化：安装后在 web/admin-vben 运行 pnpm format。")
			}
			fmt.Println("已注册路由与 OpenAPI 操作。下一步：make migrate.up，然后按业务补充校验规则、字段中文名与菜单图标。")
			return nil
		},
	}
	cmd.Flags().StringVar(&fieldSpec, "fields", "", `字段列表，默认 "`+defaultFields+`"`)
	cmd.Flags().StringVar(&label, "label", "", "模块显示名，用于权限名与接口分组，默认同模块名")
	return cmd
}
