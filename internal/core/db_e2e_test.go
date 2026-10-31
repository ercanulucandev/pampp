package core

import (
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// TestDatabaseManagement exercises the Veritabanı page backend against the MariaDB
// installed in PAMPP_E2E_ROOT (run TestEndToEnd once first to install it).
func TestDatabaseManagement(t *testing.T) {
	root := os.Getenv("PAMPP_E2E_ROOT")
	if root == "" {
		t.Skip("set PAMPP_E2E_ROOT to run")
	}
	InitJobObject()
	m := NewManager(root, nil)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(m.updateConfig(func(c *Config) { c.DBPort = 3399; c.DBPassword = "" }))
	must(m.StartService("db"))
	defer m.Shutdown()

	// Clean slate from earlier runs.
	for _, d := range []string{"shop", "shop_copy"} {
		m.RunSQL("", "DROP DATABASE IF EXISTS "+d)
	}
	m.RunSQL("", "DROP USER IF EXISTS 'app'@'%'")

	// databases, tables, rows, SQL console
	must(m.CreateDatabase("shop"))
	if err := m.CreateDatabase("shop"); err == nil || !strings.Contains(err.Error(), "zaten var") {
		t.Fatalf("duplicate create: %v", err)
	}
	if err := m.CreateDatabase("bad`name"); err == nil {
		t.Fatal("identifier with backtick accepted")
	}
	if err := m.DropDatabase("mysql"); err == nil {
		t.Fatal("system database drop allowed")
	}
	_, err := m.RunSQL("shop", "CREATE TABLE products (id INT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(50), note TEXT NULL)")
	must(err)
	res, err := m.RunSQL("shop", "INSERT INTO products (name) VALUES ('Kalem'), ('Defter'), ('Silgi');")
	must(err)
	if res.Affected != 3 {
		t.Fatalf("affected = %d", res.Affected)
	}
	tables, err := m.DBTables("shop")
	must(err)
	if len(tables) != 1 || tables[0].Name != "products" {
		t.Fatalf("tables: %+v", tables)
	}
	rows, err := m.DBRows("shop", "products")
	must(err)
	if len(rows.Rows) != 3 || *rows.Rows[1][1] != "Defter" || rows.Rows[0][2] != nil {
		t.Fatalf("rows: %+v", rows)
	}
	sel, err := m.RunSQL("shop", "SELECT COUNT(*) AS n FROM products")
	must(err)
	if !sel.IsQuery || *sel.Rows[0][0] != "3" {
		t.Fatalf("select: %+v", sel)
	}
	ov, err := m.DBOverview()
	must(err)
	t.Logf("server %s, %d databases", ov.Version, len(ov.Databases))

	// users: the default host must accept TCP clients on 127.0.0.1
	must(m.CreateDBUser("app", "", "p@ss'1", "shop"))
	connect := func(user, pw, dbname string) error {
		c := mysql.NewConfig()
		c.User, c.Passwd, c.Net, c.Addr, c.DBName = user, pw, "tcp", "127.0.0.1:3399", dbname
		db, err := sql.Open("mysql", c.FormatDSN())
		if err != nil {
			return err
		}
		defer db.Close()
		var n int
		return db.QueryRow("SELECT COUNT(*) FROM products").Scan(&n)
	}
	must(connect("app", "p@ss'1", "shop"))
	if err := connect("app", "p@ss'1", "mysql"); err == nil {
		t.Fatal("app user can read a database it was not granted")
	}
	users, err := m.DBUsers()
	must(err)
	found := false
	for _, u := range users {
		found = found || (u.User == "app" && u.Host == "%" && len(u.Grants) > 0)
	}
	if !found {
		t.Fatalf("users: %+v", users)
	}
	must(m.SetDBUserPassword("app", "%", "new-pass"))
	must(connect("app", "new-pass", "shop"))
	if err := m.DropDBUser("root", "localhost"); err == nil {
		t.Fatal("root drop allowed")
	}
	must(m.DropDBUser("app", "%"))

	// backup, then import into another database
	path, err := m.ExportDatabase("shop")
	must(err)
	defer os.Remove(path)
	dump, _ := os.ReadFile(path)
	if !strings.Contains(string(dump), "CREATE TABLE `products`") || !strings.Contains(string(dump), "Defter") ||
		strings.Contains(string(dump), "USE `shop`") {
		t.Fatalf("dump content unexpected (%d bytes)", len(dump))
	}
	id, err := m.ImportSQL("shop_copy", path)
	must(err)
	deadline := time.Now().Add(60 * time.Second)
	for {
		m.mu.Lock()
		task := m.tasks[id]
		done, terr := task != nil && task.Done, ""
		if task != nil {
			terr = task.Error
		}
		m.mu.Unlock()
		if done {
			if terr != "" {
				t.Fatal(terr)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("import timed out")
		}
		time.Sleep(200 * time.Millisecond)
	}
	copyRows, err := m.DBRows("shop_copy", "products")
	must(err)
	if len(copyRows.Rows) != 3 {
		t.Fatalf("imported rows = %d", len(copyRows.Rows))
	}
	t.Logf("backup imported into shop_copy")

	// root password with a quote and backslash: PAMPP, shutdown and phpMyAdmin must follow
	const pw = `s3cr3t'pw\x`
	must(m.SetRootPassword(pw))
	if _, err := m.DBOverview(); err != nil {
		t.Fatalf("overview after password change: %v", err)
	}
	if m.componentInstalled("phpmyadmin") {
		conf, _ := os.ReadFile(m.P.Bin("phpmyadmin", "config.inc.php"))
		if !strings.Contains(string(conf), `'s3cr3t\'pw\\x'`) {
			t.Fatalf("phpMyAdmin password not written/escaped:\n%s", conf)
		}
	}
	start := time.Now()
	m.StopService("db")
	if took := time.Since(start); took > 12*time.Second {
		t.Fatalf("graceful shutdown with password failed (took %s, fell back to kill)", took)
	}
	must(m.StartService("db"))
	if _, err := m.DBOverview(); err != nil {
		t.Fatalf("after restart: %v", err)
	}
	must(m.SetRootPassword(""))
	m.RunSQL("", "DROP DATABASE shop")
	m.RunSQL("", "DROP DATABASE shop_copy")
	t.Log("root password change, graceful shutdown and restart OK")
}
