package migrate

import (
	"fmt"

	"gorm.io/gorm"
)

// Rollbacks run with application writes stopped. Check data compatibility
// before the migration engine marks the version dirty or executes any DDL;
// MySQL DDL cannot be rolled back as one transaction.
func validateDownMigration(db *gorm.DB, name string) error {
	switch name {
	case "202604150009_define_integrity_semantics":
		return validateLegacyUniqueness(db)
	case "202604150011_add_system_config_secrets":
		if db == nil {
			return fmt.Errorf("migration database is required")
		}
		var hasSecrets bool
		if err := db.Raw(`SELECT EXISTS (SELECT 1 FROM system_configs WHERE is_secret = TRUE)`).Scan(&hasSecrets).Error; err != nil {
			return fmt.Errorf("check encrypted system configs before down migration: %w", err)
		}
		if hasSecrets {
			return fmt.Errorf("cannot remove is_secret while encrypted system configs exist")
		}
	}
	return nil
}

func validateLegacyUniqueness(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	checks := []struct{ table, columns, predicate string }{
		{"users", "email", "email IS NOT NULL"},
		{"console_roles", "code", "code IS NOT NULL"},
		{"console_admins", "account", "account IS NOT NULL"},
		{"system_configs", "config_group, config_key", "config_group IS NOT NULL AND config_key IS NOT NULL"},
	}
	if db.Name() == "postgres" {
		checks = append(checks, struct{ table, columns, predicate string }{"console_admins", "email", "email <> ''"},
			struct{ table, columns, predicate string }{"console_admins", "phone", "phone <> ''"})
	}
	for _, check := range checks {
		// Identifiers and predicates are fixed above, never user input. Include
		// soft-deleted rows because the old indexes are not scoped to active rows.
		query := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s WHERE %s GROUP BY %s HAVING COUNT(*) > 1)", check.table, check.predicate, check.columns)
		var duplicates bool
		if err := db.Raw(query).Scan(&duplicates).Error; err != nil {
			return fmt.Errorf("check legacy uniqueness on %s before rollback: %w", check.table, err)
		}
		if duplicates {
			return fmt.Errorf("cannot restore legacy uniqueness on %s (%s): conflicting rows include soft-deleted data; reconcile data explicitly before rollback", check.table, check.columns)
		}
	}
	return nil
}
