package tdb

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ---- 极简内存驱动，仅供测试（不进生产代码） ----

func init() {
	sql.Register("tdb_mem", &memDriver{})
}

type memDriver struct{}

func (d *memDriver) Open(name string) (driver.Conn, error) {
	return &memConn{name: name}, nil
}

// memStore 全局内存表存储（按连接名隔离）。
var (
	memMu    sync.Mutex
	memStore = map[string]map[string][]map[string]any{} // conn -> table -> rows
)

type memConn struct {
	name string
}

func (c *memConn) Prepare(query string) (driver.Stmt, error) {
	return &memStmt{conn: c, query: query}, nil
}
func (c *memConn) Close() error              { return nil }
func (c *memConn) Begin() (driver.Tx, error) { return &memTx{}, nil }

type memTx struct{}

func (t *memTx) Commit() error   { return nil }
func (t *memTx) Rollback() error { return nil }

// tableRef 从表名（可能带 ` 或 " 引用）提取裸名。
func tableRef(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`\"")
	return s
}

type memStmt struct {
	conn  *memConn
	query string
}

func (s *memStmt) Close() error  { return nil }
func (s *memStmt) NumInput() int { return -1 }

func (s *memStmt) Exec(args []driver.Value) (driver.Result, error) {
	q := strings.TrimSpace(s.query)
	switch {
	case strings.HasPrefix(strings.ToUpper(q), "INSERT"):
		return s.execInsert(q, args)
	case strings.HasPrefix(strings.ToUpper(q), "UPDATE"):
		return s.execUpdate(q, args)
	case strings.HasPrefix(strings.ToUpper(q), "DELETE"):
		return s.execDelete(q, args)
	default:
		return nil, fmt.Errorf("memdriver: unsupported exec %q", q)
	}
}

func (s *memStmt) Query(args []driver.Value) (driver.Rows, error) {
	q := strings.TrimSpace(s.query)
	if strings.HasPrefix(strings.ToUpper(q), "SELECT") {
		return s.querySelect(q, args)
	}
	return nil, fmt.Errorf("memdriver: unsupported query %q", q)
}

// 解析 WHERE 子句中的条件（支持 col OP ? 形式，OP ∈ = > < >= <= <> !=）。
func parseWhere(where string, args []driver.Value) ([]func(map[string]any) bool, error) {
	where = strings.TrimSpace(where)
	if where == "" {
		return nil, nil
	}
	var conds []func(map[string]any) bool
	parts := strings.Split(where, " AND ")
	ai := 0
	for _, p := range parts {
		p = strings.TrimSpace(p)
		op := "="
		opPos := strings.Index(p, "= ?")
		if i := strings.Index(p, ">= ?"); i >= 0 {
			op, opPos = ">=", i
		} else if i := strings.Index(p, "<= ?"); i >= 0 {
			op, opPos = "<=", i
		} else if i := strings.Index(p, "<> ?"); i >= 0 {
			op, opPos = "<>", i
		} else if i := strings.Index(p, "!= ?"); i >= 0 {
			op, opPos = "!=", i
		} else if i := strings.Index(p, "> ?"); i >= 0 {
			op, opPos = ">", i
		} else if i := strings.Index(p, "< ?"); i >= 0 {
			op, opPos = "<", i
		} else if i := strings.Index(p, "= ?"); i >= 0 {
			op, opPos = "=", i
		}
		if opPos < 0 {
			continue
		}
		col := strings.Trim(strings.TrimSpace(p[:opPos]), "`\"")
		if ai >= len(args) {
			return nil, fmt.Errorf("memdriver: arg mismatch")
		}
		v := args[ai]
		ai++
		colCopy := col
		valCopy := v
		opCopy := op
		conds = append(conds, func(row map[string]any) bool {
			lv := fmt.Sprintf("%v", row[colCopy])
			rv := fmt.Sprintf("%v", valCopy)
			switch opCopy {
			case "=":
				return lv == rv
			case ">":
				return lv > rv
			case "<":
				return lv < rv
			case ">=":
				return lv >= rv
			case "<=":
				return lv <= rv
			case "<>", "!=":
				return lv != rv
			}
			return false
		})
	}
	return conds, nil
}

func (s *memStmt) querySelect(q string, args []driver.Value) (driver.Rows, error) {
	memMu.Lock()
	defer memMu.Unlock()
	// SELECT cols FROM table [WHERE ...] [ORDER BY ...] [LIMIT n OFFSET m]
	upper := strings.ToUpper(q)
	fromIdx := strings.Index(upper, " FROM ")
	colsPart := strings.TrimSpace(q[len("SELECT "):fromIdx])
	rest := q[fromIdx+len(" FROM "):]

	table := rest
	var where string
	var orderBy string
	var limit int
	var offset int
	if wi := strings.Index(strings.ToUpper(rest), " WHERE "); wi >= 0 {
		table = rest[:wi]
		after := rest[wi+len(" WHERE "):]
		// 截取 where 直到 ORDER/LIMIT（取二者中较早者）
		end := len(after)
		if oi := strings.Index(strings.ToUpper(after), " ORDER BY "); oi >= 0 {
			if oi < end {
				end = oi
			}
			orderBy = strings.TrimSpace(after[oi+len(" ORDER BY "):])
		}
		if li := strings.Index(strings.ToUpper(after), " LIMIT "); li >= 0 {
			if li < end {
				end = li
			}
			seg := after[li+len(" LIMIT "):]
			if sp := strings.Index(seg, " OFFSET "); sp >= 0 {
				limit, _ = strconv.Atoi(strings.TrimSpace(seg[:sp]))
				offset, _ = strconv.Atoi(strings.TrimSpace(seg[sp+len(" OFFSET "):]))
			} else {
				limit, _ = strconv.Atoi(strings.TrimSpace(seg))
			}
		}
		where = strings.TrimSpace(after[:end])
	} else {
		if oi := strings.Index(strings.ToUpper(rest), " ORDER BY "); oi >= 0 {
			table = rest[:oi]
			orderBy = strings.TrimSpace(rest[oi+len(" ORDER BY "):])
		}
		if li := strings.Index(strings.ToUpper(rest), " LIMIT "); li >= 0 {
			// 保留 table 前缀，但解析并应用 limit/offset
			seg := rest[li+len(" LIMIT "):]
			if sp := strings.Index(seg, " OFFSET "); sp >= 0 {
				limit, _ = strconv.Atoi(strings.TrimSpace(seg[:sp]))
				offset, _ = strconv.Atoi(strings.TrimSpace(seg[sp+len(" OFFSET "):]))
			} else {
				limit, _ = strconv.Atoi(strings.TrimSpace(seg))
			}
			// 移除 LIMIT 段对 table 的影响（table 已在 ORDER BY 前截好）
		}
	}
	table = tableRef(table)

	rows, ok := memStore[s.conn.name][table]
	if !ok {
		rows = []map[string]any{}
	}

	conds, err := parseWhere(where, args)
	if err != nil {
		return nil, err
	}
	var filtered []map[string]any
	for _, r := range rows {
		match := true
		for _, c := range conds {
			if !c(r) {
				match = false
				break
			}
		}
		if match {
			filtered = append(filtered, r)
		}
	}

	if orderBy != "" {
		ob := strings.TrimSpace(strings.Split(orderBy, " LIMIT ")[0])
		desc := false
		if strings.HasSuffix(strings.ToUpper(ob), " DESC") {
			desc = true
			ob = strings.TrimSpace(ob[:len(ob)-len("DESC")])
		}
		ob = tableRef(ob)
		sort.SliceStable(filtered, func(i, j int) bool {
			vi := fmt.Sprintf("%v", filtered[i][ob])
			vj := fmt.Sprintf("%v", filtered[j][ob])
			if desc {
				return vi > vj
			}
			return vi < vj
		})
	}

	if offset > 0 && offset < len(filtered) {
		filtered = filtered[offset:]
	} else if offset >= len(filtered) {
		filtered = nil
	}
	if limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	// 决定输出列
	var outCols []string
	if strings.Contains(strings.ToUpper(colsPart), "COUNT(") {
		// 聚合：返回单行单列计数
		outCols = []string{"COUNT(*)"}
		return &memRows{cols: outCols, data: []map[string]any{{"COUNT(*)": len(filtered)}}}, nil
	}
	if colsPart == "*" {
		if len(filtered) > 0 {
			for k := range filtered[0] {
				outCols = append(outCols, k)
			}
			sort.Strings(outCols)
		} else {
			outCols = []string{"id"}
		}
	} else {
		for _, c := range strings.Split(colsPart, ",") {
			outCols = append(outCols, tableRef(strings.TrimSpace(c)))
		}
	}

	return &memRows{cols: outCols, data: filtered}, nil
}

type memRows struct {
	cols []string
	data []map[string]any
	pos  int
}

func (r *memRows) Columns() []string { return r.cols }
func (r *memRows) Close() error      { return nil }
func (r *memRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.data) {
		return io.EOF
	}
	row := r.data[r.pos]
	r.pos++
	for i, c := range r.cols {
		v, ok := row[c]
		if !ok {
			dest[i] = nil
			continue
		}
		dest[i] = driver.Value(v)
	}
	return nil
}

func (s *memStmt) execInsert(q string, args []driver.Value) (driver.Result, error) {
	memMu.Lock()
	defer memMu.Unlock()
	// INSERT INTO t (c1, c2) VALUES (?, ?)[, (?, ?)]...  支持单值/多值。
	if memStore[s.conn.name] == nil {
		memStore[s.conn.name] = map[string][]map[string]any{}
	}
	// 表名：截到第一个 " (" 之前。
	paren := strings.Index(q, " (")
	table := tableRef(strings.TrimSpace(q[len("INSERT INTO "):paren]))
	colsPart := q[paren+2 : strings.Index(q, ") VALUES")]
	var cols []string
	for _, c := range strings.Split(colsPart, ",") {
		cols = append(cols, tableRef(strings.TrimSpace(c)))
	}

	// 解析所有 (?,?) 分组
	valuesSection := q[strings.Index(q, ") VALUES")+len(") VALUES"):]
	var groups []string
	depth := 0
	start := -1
	for i, r := range valuesSection {
		switch r {
		case '(':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case ')':
			depth--
			if depth == 0 {
				groups = append(groups, valuesSection[start:i])
				start = -1
			}
		}
	}

	n := 0
	argIdx := 0
	for _, g := range groups {
		parts := strings.Split(g, ",")
		row := map[string]any{}
		for i, c := range cols {
			if argIdx >= len(args) {
				break
			}
			row[c] = stripVal(args[argIdx])
			argIdx++
			_ = parts[i]
		}
		// 自增 id 模拟
		maxID := 0
		for _, r := range memStore[s.conn.name][table] {
			if id, ok := r["id"].(int); ok && id > maxID {
				maxID = id
			}
		}
		if _, ok := row["id"]; !ok {
			row["id"] = maxID + 1
		}
		memStore[s.conn.name][table] = append(memStore[s.conn.name][table], row)
		n++
	}
	return &memResult{lastID: 0, rows: n}, nil
}

func (s *memStmt) execUpdate(q string, args []driver.Value) (driver.Result, error) {
	memMu.Lock()
	defer memMu.Unlock()
	// UPDATE t SET c1 = ?, c2 = ? WHERE ...
	upper := strings.ToUpper(q)
	setStart := strings.Index(upper, " SET ") + len(" SET ")
	whereStart := len(q)
	if wi := strings.Index(upper, " WHERE "); wi >= 0 {
		whereStart = wi
	}
	table := tableRef(strings.TrimSpace(q[len("UPDATE "):strings.Index(upper, " SET ")]))
	setPart := q[setStart:whereStart]
	where := ""
	if whereStart < len(q) {
		where = q[whereStart+len(" WHERE "):]
	}
	// 解析 SET 段：col = ? 按顺序取 args
	setCols := []string{}
	segs := strings.Split(setPart, ",")
	for _, seg := range segs {
		eq := strings.Index(seg, "= ?")
		if eq > 0 {
			setCols = append(setCols, tableRef(strings.TrimSpace(seg[:eq])))
		}
	}
	// WHERE args 跟在 SET args 之后
	var whereArgs []driver.Value
	if len(where) > 0 {
		wcount := strings.Count(where, "?")
		whereArgs = args[len(args)-wcount:]
	}
	conds, err := parseWhere(where, whereArgs)
	if err != nil {
		return nil, err
	}
	rows := memStore[s.conn.name][table]
	affected := 0
	for _, r := range rows {
		match := true
		for _, c := range conds {
			if !c(r) {
				match = false
				break
			}
		}
		if match {
			for i, c := range setCols {
				r[c] = stripVal(args[i])
			}
			affected++
		}
	}
	return &memResult{rows: affected}, nil
}

func (s *memStmt) execDelete(q string, args []driver.Value) (driver.Result, error) {
	memMu.Lock()
	defer memMu.Unlock()
	upper := strings.ToUpper(q)
	// 表名：截到 WHERE 之前（无 WHERE 则取整段去尾部空格）。
	tableEnd := len(q)
	if wi := strings.Index(upper, " WHERE "); wi >= 0 {
		tableEnd = wi
	}
	table := tableRef(strings.TrimSpace(q[len("DELETE FROM "):tableEnd]))
	var where string
	if wi := strings.Index(upper, " WHERE "); wi >= 0 {
		where = q[wi+len(" WHERE "):]
	}
	conds, err := parseWhere(where, args)
	if err != nil {
		return nil, err
	}
	rows := memStore[s.conn.name][table]
	var kept []map[string]any
	affected := 0
	for _, r := range rows {
		match := true
		for _, c := range conds {
			if !c(r) {
				match = false
				break
			}
		}
		if match {
			affected++
		} else {
			kept = append(kept, r)
		}
	}
	memStore[s.conn.name][table] = kept
	return &memResult{rows: affected}, nil
}

type memResult struct {
	lastID int
	rows   int
}

func (r *memResult) LastInsertId() (int64, error) { return int64(r.lastID), nil }
func (r *memResult) RowsAffected() (int64, error) { return int64(r.rows), nil }

func stripVal(v driver.Value) any {
	if v == nil {
		return nil
	}
	return v
}

// seedTable 测试辅助：向内存库预置数据。
func seedTable(conn, table string, rows ...map[string]any) {
	memMu.Lock()
	defer memMu.Unlock()
	if memStore[conn] == nil {
		memStore[conn] = map[string][]map[string]any{}
	}
	memStore[conn][table] = rows
}
