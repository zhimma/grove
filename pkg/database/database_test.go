package database

import (
	"net/url"
	"strings"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

func TestConnectionsSupportNamedResources(t *testing.T) {
	defaultDB := &gorm.DB{}
	ordersDB := &gorm.DB{}

	dbs := NewConnectionsFromDBs(defaultDB, map[string]*gorm.DB{
		"orders": ordersDB,
	})

	if got := dbs.Default(); got != defaultDB {
		t.Fatal("expected default database to match")
	}

	got, err := dbs.Get("orders")
	if err != nil {
		t.Fatalf("get orders db: %v", err)
	}
	if got != ordersDB {
		t.Fatal("expected orders database to match")
	}

	if !dbs.Has("default") || !dbs.Has("orders") {
		t.Fatal("expected dbs to report configured resources")
	}
}

func TestBuildMySQLDSNEscapesCredentialsAndEnablesCompatibilityOptions(t *testing.T) {
	dsn := buildMySQLDSN(Config{
		Host:      "127.0.0.1",
		Port:      "3306",
		User:      "root@example",
		Password:  "p@ss/word?&",
		DBName:    "grove",
		ParseTime: true,
		Loc:       "UTC",
		Charset:   "utf8mb4",
		TLS:       true,
	})
	for _, fragment := range []string{
		"charset=utf8mb4",
		"parseTime=true",
		"loc=UTC",
		"multiStatements=true",
		"tls=true",
	} {
		if !strings.Contains(dsn, fragment) {
			t.Fatalf("mysql DSN missing %q: %s", fragment, dsn)
		}
	}
	parsed, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse generated mysql DSN: %v", err)
	}
	if parsed.User != "root@example" || parsed.Passwd != "p@ss/word?&" || parsed.DBName != "grove" {
		t.Fatalf("credentials were not preserved: %#v", parsed)
	}
}

func TestBuildPostgresDSNEscapesCredentialsAndDatabaseName(t *testing.T) {
	dsn := buildPostgresDSN(Config{
		Host:     "db.example",
		Port:     "5432",
		User:     "grove user",
		Password: `p@ss word/?&`,
		DBName:   "grove db",
		SSLMode:  "verify-full",
	})
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse generated postgres DSN: %v", err)
	}
	if parsed.Scheme != "postgres" || parsed.Host != "db.example:5432" {
		t.Fatalf("unexpected postgres DSN endpoint: %s", dsn)
	}
	if got := parsed.Query().Get("dbname"); got != "grove db" {
		t.Fatalf("database name was not preserved: %q", got)
	}
	if got := parsed.Query().Get("sslmode"); got != "verify-full" {
		t.Fatalf("ssl mode was not preserved: %q", got)
	}
	user := parsed.User.Username()
	password, ok := parsed.User.Password()
	if !ok || user != "grove user" || password != `p@ss word/?&` {
		t.Fatalf("credentials were not preserved: user=%q password=%q ok=%v", user, password, ok)
	}
}
