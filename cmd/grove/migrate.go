package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zhimma/grove/pkg/migrate"
)

func newMigrateCmd() *cobra.Command {
	migrationPath := "database/migrations"
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "管理 SQL 迁移",
	}
	cmd.PersistentFlags().StringVar(&migrationPath, "path", migrationPath, "迁移文件目录")

	cmd.AddCommand(&cobra.Command{
		Use:   "up",
		Short: "执行所有待执行迁移",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, cleanup, err := openDefaultDB()
			if err != nil {
				return err
			}
			defer cleanup()

			m := migrate.NewMigrator(db, migrationPath)
			count, err := m.Up()
			if err != nil {
				return err
			}
			fmt.Printf("已执行 %d 个迁移文件\n", count)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "down",
		Short: "回滚最近一次迁移",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, cleanup, err := openDefaultDB()
			if err != nil {
				return err
			}
			defer cleanup()

			m := migrate.NewMigrator(db, migrationPath)
			name, err := m.Down()
			if err != nil {
				return err
			}
			if name == "" {
				fmt.Println("没有可回滚的迁移")
				return nil
			}
			fmt.Printf("已回滚迁移 %s\n", name)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "查看迁移状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, cleanup, err := openDefaultDB()
			if err != nil {
				return err
			}
			defer cleanup()

			m := migrate.NewMigrator(db, migrationPath)
			statuses, err := m.Status()
			if err != nil {
				return err
			}
			if len(statuses) == 0 {
				fmt.Println("未找到迁移文件")
				return nil
			}
			for _, status := range statuses {
				state := "待执行"
				if status.Applied {
					state = "已执行"
				}
				fmt.Printf("%-10s %s\n", state, status.Name)
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "create [name]",
		Short: "为 postgres 与 mysql 各创建一对同版本的迁移文件",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := migrate.CreateFiles(migrationPath, args[0], nil)
			if err != nil {
				return err
			}
			for _, path := range paths {
				fmt.Println(path)
			}
			return nil
		},
	})

	return cmd
}
