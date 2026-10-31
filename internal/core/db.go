package core

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

var (
	identRe = regexp.MustCompile(`^[A-Za-z0-9_$-]{1,64}$`)
	userRe  = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,80}$`)
	hostRe  = regexp.MustCompile(`^[A-Za-z0-9_.%:-]{1,255}$`)
)

var systemDatabases = map[string]bool{"mysql": true, "information_schema": true, "performance_schema": true, "sys": true}

func quoteIdent(s string) string { return "`" + strings.ReplaceAll(s, "`", "``") + "`" }

func quoteString(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`, "\x00", `\0`).Replace(s) + "'"
}

func checkIdent(kind, s string) error {
	if !identRe.MatchString(s) {
		return fmt.Errorf("geçersiz %s adı: harf, rakam, _ ve - kullanın", kind)
	}
	return nil
}

func checkAccount(user, host string) error {
	if !userRe.MatchString(user) {
		return fmt.Errorf("geçersiz kullanıcı adı")
	}
	if !hostRe.MatchString(host) {
		return fmt.Errorf("geçersiz host")
	}
	return nil
}

func account(user, host string) string { return quoteString(user) + "@" + quoteString(host) }

// sqlDB opens a connection to the running MariaDB as root.
func (m *Manager) sqlDB(database string) (*sql.DB, error) {
	if !m.dbRunning() {
		return nil, fmt.Errorf("MariaDB çalışmıyor, önce başlatın")
	}
	cfg := m.config()
	c := mysql.NewConfig()
	c.User = "root"
	c.Passwd = cfg.DBPassword
	c.Net = "tcp"
	c.Addr = fmt.Sprintf("127.0.0.1:%d", cfg.DBPort)
	c.DBName = database
	c.Timeout = 3 * time.Second
	c.Params = map[string]string{"charset": "utf8mb4"}
	db, err := sql.Open("mysql", c.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func (m *Manager) withDB(database string, fn func(db *sql.DB) error) error {
	db, err := m.sqlDB(database)
	if err != nil {
		return err
	}
	defer db.Close()
	return fn(db)
}

// ---------------- overview ----------------

type DBInfo struct {
	Name   string `json:"name"`
	Tables int    `json:"tables"`
	Size   int64  `json:"size"`
	System bool   `json:"system"`
}

type DBOverview struct {
	Version   string   `json:"version"`
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	User      string   `json:"user"`
	Password  string   `json:"password"`
	Databases []DBInfo `json:"databases"`
}

func (m *Manager) DBOverview() (*DBOverview, error) {
	cfg := m.config()
	ov := &DBOverview{Host: "127.0.0.1", Port: cfg.DBPort, User: "root", Password: cfg.DBPassword, Databases: []DBInfo{}}
	err := m.withDB("", func(db *sql.DB) error {
		if err := db.QueryRow("SELECT VERSION()").Scan(&ov.Version); err != nil {
			return dbError(err)
		}
		names, err := queryStrings(db, "SHOW DATABASES")
		if err != nil {
			return err
		}
		stats := map[string]DBInfo{}
		rows, err := db.Query(`SELECT table_schema, COUNT(*), COALESCE(SUM(data_length + index_length), 0)
			FROM information_schema.tables GROUP BY table_schema`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var d DBInfo
			if err := rows.Scan(&d.Name, &d.Tables, &d.Size); err == nil {
				stats[d.Name] = d
			}
		}
		for _, n := range names {
			d := stats[n]
			d.Name, d.System = n, systemDatabases[n]
			ov.Databases = append(ov.Databases, d)
		}
		sort.SliceStable(ov.Databases, func(i, j int) bool {
			if ov.Databases[i].System != ov.Databases[j].System {
				return !ov.Databases[i].System
			}
			return ov.Databases[i].Name < ov.Databases[j].Name
		})
		return nil
	})
	return ov, err
}

func queryStrings(db *sql.DB, q string, args ...any) ([]string, error) {
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// dbError turns driver errors into short Turkish messages where it helps.
func dbError(err error) error {
	if err == nil {
		return nil
	}
	if me, ok := err.(*mysql.MySQLError); ok {
		switch me.Number {
		case 1045:
			return fmt.Errorf("MariaDB root şifresi kabul edilmedi")
		case 1007:
			return fmt.Errorf("bu veritabanı zaten var")
		case 1396:
			return fmt.Errorf("bu kullanıcı zaten var ya da bulunamadı")
		}
		return fmt.Errorf("%s", me.Message)
	}
	return err
}

// ---------------- databases & tables ----------------

func (m *Manager) CreateDatabase(name string) error {
	if err := checkIdent("veritabanı", name); err != nil {
		return err
	}
	return m.withDB("", func(db *sql.DB) error {
		_, err := db.Exec("CREATE DATABASE " + quoteIdent(name) + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
		return dbError(err)
	})
}

func (m *Manager) DropDatabase(name string) error {
	if err := checkIdent("veritabanı", name); err != nil {
		return err
	}
	if systemDatabases[name] {
		return fmt.Errorf("%s bir sistem veritabanı, silinemez", name)
	}
	return m.withDB("", func(db *sql.DB) error {
		_, err := db.Exec("DROP DATABASE " + quoteIdent(name))
		return dbError(err)
	})
}

type TableInfo struct {
	Name      string `json:"name"`
	Engine    string `json:"engine"`
	Rows      int64  `json:"rows"`
	Size      int64  `json:"size"`
	Collation string `json:"collation"`
	Kind      string `json:"kind"` // BASE TABLE | VIEW
}

func (m *Manager) DBTables(database string) ([]TableInfo, error) {
	if err := checkIdent("veritabanı", database); err != nil {
		return nil, err
	}
	out := []TableInfo{}
	err := m.withDB("", func(db *sql.DB) error {
		rows, err := db.Query(`SELECT table_name, COALESCE(engine, ''), COALESCE(table_rows, 0),
			COALESCE(data_length + index_length, 0), COALESCE(table_collation, ''), table_type
			FROM information_schema.tables WHERE table_schema = ? ORDER BY table_name`, database)
		if err != nil {
			return dbError(err)
		}
		defer rows.Close()
		for rows.Next() {
			var t TableInfo
			if err := rows.Scan(&t.Name, &t.Engine, &t.Rows, &t.Size, &t.Collation, &t.Kind); err != nil {
				return err
			}
			out = append(out, t)
		}
		return rows.Err()
	})
	return out, err
}

type QueryResult struct {
	Columns   []string    `json:"columns"`
	Rows      [][]*string `json:"rows"`
	Affected  int64       `json:"affected"`
	Truncated bool        `json:"truncated"`
	Millis    int64       `json:"millis"`
	IsQuery   bool        `json:"isQuery"`
}

const maxResultRows = 1000

func scanResult(rows *sql.Rows, limit int) (*QueryResult, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	res := &QueryResult{Columns: cols, Rows: [][]*string{}, IsQuery: true}
	raw := make([]sql.RawBytes, len(cols))
	ptrs := make([]any, len(cols))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	for rows.Next() {
		if len(res.Rows) >= limit {
			res.Truncated = true
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make([]*string, len(cols))
		for i, b := range raw {
			if b == nil {
				continue
			}
			s := string(b)
			if len(s) > 500 {
				s = s[:500] + "…"
			}
			row[i] = &s
		}
		res.Rows = append(res.Rows, row)
	}
	return res, rows.Err()
}

func (m *Manager) DBRows(database, table string) (*QueryResult, error) {
	if err := checkIdent("veritabanı", database); err != nil {
		return nil, err
	}
	if err := checkIdent("tablo", table); err != nil {
		return nil, err
	}
	var res *QueryResult
	err := m.withDB(database, func(db *sql.DB) error {
		start := time.Now()
		rows, err := db.Query("SELECT * FROM " + quoteIdent(table) + " LIMIT " + strconv.Itoa(200+1))
		if err != nil {
			return dbError(err)
		}
		defer rows.Close()
		res, err = scanResult(rows, 200)
		if res != nil {
			res.Millis = time.Since(start).Milliseconds()
		}
		return err
	})
	return res, err
}

func (m *Manager) TableAction(database, table, action string) error {
	if err := checkIdent("veritabanı", database); err != nil {
		return err
	}
	if err := checkIdent("tablo", table); err != nil {
		return err
	}
	if systemDatabases[database] {
		return fmt.Errorf("sistem veritabanında değişiklik yapılamaz")
	}
	stmt := map[string]string{"truncate": "TRUNCATE TABLE ", "drop": "DROP TABLE "}[action]
	if stmt == "" {
		return fmt.Errorf("bilinmeyen işlem")
	}
	return m.withDB(database, func(db *sql.DB) error {
		_, err := db.Exec(stmt + quoteIdent(table))
		return dbError(err)
	})
}

// RunSQL executes a statement typed into the SQL console.
func (m *Manager) RunSQL(database, query string) (*QueryResult, error) {
	query = strings.TrimSpace(query)
	query = strings.TrimSuffix(query, ";")
	if query == "" {
		return nil, fmt.Errorf("sorgu boş")
	}
	if database != "" {
		if err := checkIdent("veritabanı", database); err != nil {
			return nil, err
		}
	}
	var res *QueryResult
	err := m.withDB(database, func(db *sql.DB) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		start := time.Now()
		first := strings.ToLower(strings.Fields(query)[0])
		switch first {
		case "select", "show", "describe", "desc", "explain", "with", "values", "table", "check", "analyze", "help":
			rows, err := db.QueryContext(ctx, query)
			if err != nil {
				return dbError(err)
			}
			defer rows.Close()
			if res, err = scanResult(rows, maxResultRows); err != nil {
				return err
			}
		default:
			r, err := db.ExecContext(ctx, query)
			if err != nil {
				return dbError(err)
			}
			n, _ := r.RowsAffected()
			res = &QueryResult{Columns: []string{}, Rows: [][]*string{}, Affected: n}
		}
		res.Millis = time.Since(start).Milliseconds()
		return nil
	})
	return res, err
}

// ---------------- users ----------------

type DBUser struct {
	User   string   `json:"user"`
	Host   string   `json:"host"`
	Grants []string `json:"grants"`
	Root   bool     `json:"root"`
}

func (m *Manager) DBUsers() ([]DBUser, error) {
	out := []DBUser{}
	err := m.withDB("", func(db *sql.DB) error {
		rows, err := db.Query(`SELECT User, Host FROM mysql.user WHERE User <> '' AND User NOT IN ('mariadb.sys', 'PUBLIC') ORDER BY User, Host`)
		if err != nil {
			return dbError(err)
		}
		var users []DBUser
		for rows.Next() {
			var u DBUser
			if err := rows.Scan(&u.User, &u.Host); err != nil {
				rows.Close()
				return err
			}
			u.Root = u.User == "root"
			users = append(users, u)
		}
		rows.Close()
		for _, u := range users {
			if g, err := queryStrings(db, "SHOW GRANTS FOR "+account(u.User, u.Host)); err == nil {
				for _, line := range g {
					// Hide password hashes from the UI.
					if i := strings.Index(line, " IDENTIFIED BY PASSWORD"); i > 0 {
						line = line[:i]
					}
					u.Grants = append(u.Grants, line)
				}
			}
			out = append(out, u)
		}
		return nil
	})
	return out, err
}

// CreateDBUser creates user@host and grants it everything on database ("*" = all databases).
// The default host is "%": MariaDB runs with skip-name-resolve, so a "localhost" account
// would reject apps connecting to 127.0.0.1, and the server only listens on 127.0.0.1 anyway.
func (m *Manager) CreateDBUser(user, host, password, database string) error {
	if host == "" {
		host = "%"
	}
	if err := checkAccount(user, host); err != nil {
		return err
	}
	scope := "*.*"
	if database != "*" {
		if err := checkIdent("veritabanı", database); err != nil {
			return err
		}
		scope = quoteIdent(database) + ".*"
	}
	return m.withDB("", func(db *sql.DB) error {
		if _, err := db.Exec("CREATE USER " + account(user, host) + " IDENTIFIED BY " + quoteString(password)); err != nil {
			return dbError(err)
		}
		_, err := db.Exec("GRANT ALL PRIVILEGES ON " + scope + " TO " + account(user, host))
		return dbError(err)
	})
}

func (m *Manager) SetDBUserPassword(user, host, password string) error {
	if err := checkAccount(user, host); err != nil {
		return err
	}
	if user == "root" {
		return m.SetRootPassword(password)
	}
	return m.withDB("", func(db *sql.DB) error {
		_, err := db.Exec("ALTER USER " + account(user, host) + " IDENTIFIED BY " + quoteString(password))
		return dbError(err)
	})
}

func (m *Manager) DropDBUser(user, host string) error {
	if err := checkAccount(user, host); err != nil {
		return err
	}
	if user == "root" {
		return fmt.Errorf("root kullanıcısı silinemez")
	}
	return m.withDB("", func(db *sql.DB) error {
		_, err := db.Exec("DROP USER " + account(user, host))
		return dbError(err)
	})
}

// rootHostsUsed are the root accounts clients can actually reach. MariaDB on Windows also
// creates root@<computer name>, which skip-name-resolve never matches and ALTER USER rejects.
var rootHostsUsed = map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true, "%": true}

// SetRootPassword changes the root accounts and updates PAMPP and phpMyAdmin to match.
// If any reachable account fails, the ones already changed are put back so the
// server and PAMPP never disagree about the password.
func (m *Manager) SetRootPassword(password string) error {
	old := m.config().DBPassword
	err := m.withDB("", func(db *sql.DB) error {
		hosts, err := queryStrings(db, "SELECT Host FROM mysql.user WHERE User = 'root'")
		if err != nil {
			return err
		}
		var changed []string
		for _, h := range hosts {
			if !rootHostsUsed[h] {
				continue
			}
			if _, err := db.Exec("ALTER USER " + account("root", h) + " IDENTIFIED BY " + quoteString(password)); err != nil {
				for _, c := range changed {
					db.Exec("ALTER USER " + account("root", c) + " IDENTIFIED BY " + quoteString(old))
				}
				return fmt.Errorf("root@%s şifresi değiştirilemedi, değişiklik geri alındı: %w", h, dbError(err))
			}
			changed = append(changed, h)
		}
		if len(changed) == 0 {
			return fmt.Errorf("değiştirilecek root hesabı bulunamadı")
		}
		_, err = db.Exec("FLUSH PRIVILEGES")
		return dbError(err)
	})
	if err != nil {
		return err
	}
	if err := m.updateConfig(func(c *Config) { c.DBPassword = password }); err != nil {
		return err
	}
	m.mu.Lock()
	if m.db != nil {
		m.db.password = password
	}
	m.mu.Unlock()
	cfg := m.config()
	if m.componentInstalled("phpmyadmin") {
		_ = writePMAConfig(m.P.Bin("phpmyadmin"), cfg.DBPort, password)
	}
	m.opMu.Lock()
	_ = m.apply() // refresh the start page
	m.opMu.Unlock()
	m.changed()
	return nil
}

// ---------------- backup & import ----------------

func (m *Manager) dbTool(names ...string) (string, error) {
	for _, n := range names {
		if p := m.P.Bin("mariadb", "bin", n); exists(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s bulunamadı", names[0])
}

// ExportDatabase writes data/backups/<db>-<time>.sql and returns its path.
func (m *Manager) ExportDatabase(database string) (string, error) {
	if err := checkIdent("veritabanı", database); err != nil {
		return "", err
	}
	if !m.dbRunning() {
		return "", fmt.Errorf("MariaDB çalışmıyor, önce başlatın")
	}
	dump, err := m.dbTool("mariadb-dump.exe", "mysqldump.exe")
	if err != nil {
		return "", err
	}
	dir := m.P.Data("backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.sql", database, time.Now().Format("20060102-150405")))
	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	cfg := m.config()
	cmd := hiddenCmd(dump, "-uroot", "--host=127.0.0.1", "--port="+strconv.Itoa(cfg.DBPort),
		"--single-transaction", "--routines", "--triggers", "--events", "--default-character-set=utf8mb4", database) // no --databases: the dump imports into any database
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+cfg.DBPassword)
	cmd.Stdout = out
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err = cmd.Run()
	out.Close()
	if err != nil {
		os.Remove(path)
		return "", fmt.Errorf("yedek alınamadı: %s", lastLine(stderr.String()))
	}
	return path, nil
}

// ImportSQL runs a .sql file against database (created if missing) as a background task.
func (m *Manager) ImportSQL(database, file string) (string, error) {
	if err := checkIdent("veritabanı", database); err != nil {
		return "", err
	}
	client, err := m.dbTool("mariadb.exe", "mysql.exe")
	if err != nil {
		return "", err
	}
	if !exists(file) {
		return "", fmt.Errorf("dosya bulunamadı")
	}
	if err := m.withDB("", func(db *sql.DB) error {
		_, err := db.Exec("CREATE DATABASE IF NOT EXISTS " + quoteIdent(database) + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
		return dbError(err)
	}); err != nil {
		return "", err
	}
	t := m.newTask(filepath.Base(file) + " içe aktarılıyor")
	m.updateTask(t, func(t *Task) { t.Stage = database + " veritabanına" })
	go func() {
		err := func() error {
			in, err := os.Open(file)
			if err != nil {
				return err
			}
			defer in.Close()
			cfg := m.config()
			cmd := hiddenCmd(client, "-uroot", "--host=127.0.0.1", "--port="+strconv.Itoa(cfg.DBPort),
				"--default-character-set=utf8mb4", database)
			cmd.Env = append(os.Environ(), "MYSQL_PWD="+cfg.DBPassword)
			cmd.Stdin = in
			var stderr strings.Builder
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("içe aktarma başarısız: %s", lastLine(stderr.String()))
			}
			return nil
		}()
		if err == nil {
			m.updateTask(t, func(t *Task) { t.Label = filepath.Base(file) + " içe aktarıldı" })
		}
		m.finishTask(t, err)
	}()
	return t.ID, nil
}
