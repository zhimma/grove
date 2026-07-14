package main

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/rbac"
)

var sqlIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type rbacDifference struct {
	Kind     string
	Subject  string
	Expected string
	Actual   string
}

func newRBACCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rbac",
		Short: "检查和修复 Console RBAC 派生数据",
	}
	cmd.AddCommand(newRBACCheckCmd())
	cmd.AddCommand(newRBACRepairCmd())
	return cmd
}

func newRBACCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "检查管理员角色、grouping 和 role policy 一致性",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, enforcer, cleanup, err := openConsoleRBAC()
			if err != nil {
				return err
			}
			defer cleanup()

			differences, err := inspectConsoleRBAC(cmd.Context(), db, enforcer)
			if err != nil {
				return err
			}
			if len(differences) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "RBAC 一致性检查通过")
				return nil
			}
			printRBACDifferences(cmd.OutOrStdout(), differences)
			return fmt.Errorf("发现 %d 个 RBAC 一致性差异", len(differences))
		},
	}
}

func newRBACRepairCmd() *cobra.Command {
	dryRun := true
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "按业务表真相源修复 Console RBAC 派生数据",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, enforcer, cleanup, err := openConsoleRBAC()
			if err != nil {
				return err
			}
			defer cleanup()

			differences, err := inspectConsoleRBAC(cmd.Context(), db, enforcer)
			if err != nil {
				return err
			}
			if len(differences) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "RBAC 数据无需修复")
				return nil
			}
			printRBACDifferences(cmd.OutOrStdout(), differences)
			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "dry-run：未修改数据；使用 --dry-run=false 执行修复")
				return nil
			}
			if err := repairConsoleRBAC(cmd.Context(), db, enforcer); err != nil {
				return err
			}
			remaining, err := inspectConsoleRBAC(cmd.Context(), db, enforcer)
			if err != nil {
				return err
			}
			if len(remaining) > 0 {
				printRBACDifferences(cmd.OutOrStdout(), remaining)
				return fmt.Errorf("修复后仍存在 %d 个 RBAC 一致性差异", len(remaining))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "RBAC 修复完成")
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "只输出差异，不修改数据；设置为 false 才执行修复")
	return cmd
}

func openConsoleRBAC() (*gorm.DB, *rbac.Enforcer, func(), error) {
	cfg, err := loadCLIConfig()
	if err != nil {
		return nil, nil, nil, err
	}
	db, cleanup, err := openDefaultDBWithConfig(cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	consoleCfg := cfg.Casbin.Enforcers["console"]
	tableName := strings.TrimSpace(consoleCfg.TableName)
	if tableName == "" {
		tableName = "console_casbin_rules"
	}
	if !sqlIdentifierPattern.MatchString(tableName) {
		cleanup()
		return nil, nil, nil, fmt.Errorf("console casbin table name %q is invalid", tableName)
	}
	enforcer, err := rbac.New(db, &rbac.Config{
		Mode:      rbac.Mode(consoleCfg.Mode),
		TableName: tableName,
		ModelPath: consoleCfg.ModelPath,
	})
	if err != nil {
		cleanup()
		return nil, nil, nil, err
	}
	return db, enforcer, cleanup, nil
}

func inspectConsoleRBAC(ctx context.Context, db *gorm.DB, enforcer *rbac.Enforcer) ([]rbacDifference, error) {
	roles, admins, err := loadConsoleRBACSources(ctx, db)
	if err != nil {
		return nil, err
	}
	groupings, err := enforcer.GetGroupingPolicy()
	if err != nil {
		return nil, fmt.Errorf("load console groupings: %w", err)
	}
	policies, err := enforcer.GetPolicy()
	if err != nil {
		return nil, fmt.Errorf("load console policies: %w", err)
	}

	validRoles := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		validRoles[role.ID] = struct{}{}
	}
	expectedRoles := make(map[string]string, len(admins))
	for _, admin := range admins {
		expectedRoles[admin.ID] = admin.RoleID
	}
	actualRoles := make(map[string][]string)
	for _, grouping := range groupings {
		if len(grouping) < 2 {
			continue
		}
		actualRoles[grouping[0]] = append(actualRoles[grouping[0]], grouping[1])
	}

	differences := make([]rbacDifference, 0)
	for adminID, expectedRole := range expectedRoles {
		if _, ok := validRoles[expectedRole]; !ok {
			differences = append(differences, rbacDifference{
				Kind: "invalid_admin_role", Subject: adminID, Expected: expectedRole, Actual: "role missing",
			})
			continue
		}
		actual := uniqueSorted(actualRoles[adminID])
		if len(actual) != 1 || actual[0] != expectedRole {
			differences = append(differences, rbacDifference{
				Kind: "admin_grouping_mismatch", Subject: adminID, Expected: expectedRole, Actual: strings.Join(actual, ","),
			})
		}
	}
	for adminID, roleIDs := range actualRoles {
		if _, ok := expectedRoles[adminID]; ok {
			continue
		}
		differences = append(differences, rbacDifference{
			Kind: "orphan_grouping", Subject: adminID, Expected: "", Actual: strings.Join(uniqueSorted(roleIDs), ","),
		})
	}
	for _, policy := range policies {
		if len(policy) < 2 {
			continue
		}
		if _, ok := validRoles[policy[0]]; !ok {
			differences = append(differences, rbacDifference{
				Kind: "orphan_role_policy", Subject: policy[0], Expected: "", Actual: policy[1],
			})
		}
	}

	sort.Slice(differences, func(i, j int) bool {
		left := differences[i]
		right := differences[j]
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Subject != right.Subject {
			return left.Subject < right.Subject
		}
		return left.Actual < right.Actual
	})
	return differences, nil
}

func repairConsoleRBAC(ctx context.Context, db *gorm.DB, enforcer *rbac.Enforcer) error {
	roles, admins, err := loadConsoleRBACSources(ctx, db)
	if err != nil {
		return err
	}
	validRoles := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		validRoles[role.ID] = struct{}{}
	}
	groupings, err := enforcer.GetGroupingPolicy()
	if err != nil {
		return fmt.Errorf("load console groupings: %w", err)
	}
	actualRoles := make(map[string][]string)
	for _, grouping := range groupings {
		if len(grouping) >= 2 {
			actualRoles[grouping[0]] = append(actualRoles[grouping[0]], grouping[1])
		}
	}
	validAdmins := make(map[string]struct{}, len(admins))
	for _, admin := range admins {
		validAdmins[admin.ID] = struct{}{}
		if _, ok := validRoles[admin.RoleID]; !ok {
			continue
		}
		actual := uniqueSorted(actualRoles[admin.ID])
		if len(actual) == 1 && actual[0] == admin.RoleID {
			continue
		}
		if err := enforcer.ReplaceConsoleRoleForUser(admin.ID, admin.RoleID); err != nil {
			return fmt.Errorf("repair admin %s grouping: %w", admin.ID, err)
		}
	}
	removedAdmins := make(map[string]struct{})
	for _, grouping := range groupings {
		if len(grouping) < 1 {
			continue
		}
		if _, ok := validAdmins[grouping[0]]; ok {
			continue
		}
		if _, ok := removedAdmins[grouping[0]]; ok {
			continue
		}
		if err := enforcer.ReplaceConsoleRoleForUser(grouping[0], ""); err != nil {
			return fmt.Errorf("remove orphan grouping %s: %w", grouping[0], err)
		}
		removedAdmins[grouping[0]] = struct{}{}
	}
	policies, err := enforcer.GetPolicy()
	if err != nil {
		return fmt.Errorf("load console policies: %w", err)
	}
	orphanRoles := make(map[string]struct{})
	for _, policy := range policies {
		if len(policy) < 1 {
			continue
		}
		if _, ok := validRoles[policy[0]]; !ok {
			orphanRoles[policy[0]] = struct{}{}
		}
	}
	for roleID := range orphanRoles {
		if err := enforcer.ReplaceConsolePoliciesForRole(roleID, nil); err != nil {
			return fmt.Errorf("remove orphan role %s policies: %w", roleID, err)
		}
	}
	return nil
}

func loadConsoleRBACSources(ctx context.Context, db *gorm.DB) ([]model.ConsoleRole, []model.ConsoleAdmin, error) {
	if db == nil {
		return nil, nil, fmt.Errorf("default database is required")
	}
	var roles []model.ConsoleRole
	if err := db.WithContext(ctx).Find(&roles).Error; err != nil {
		return nil, nil, fmt.Errorf("load console roles: %w", err)
	}
	var admins []model.ConsoleAdmin
	if err := db.WithContext(ctx).Find(&admins).Error; err != nil {
		return nil, nil, fmt.Errorf("load console admins: %w", err)
	}
	return roles, admins, nil
}

func printRBACDifferences(out io.Writer, differences []rbacDifference) {
	for _, difference := range differences {
		fmt.Fprintf(out, "- %s subject=%s expected=%s actual=%s\n", difference.Kind, difference.Subject, difference.Expected, difference.Actual)
	}
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
