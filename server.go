package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type QueryRequest struct {
	Query string `json:"query"`
}

type QueryResponse struct {
	Columns   []string        `json:"columns"`
	Rows      [][]interface{} `json:"rows"`
	Message   string          `json:"message"`
	QueryType string          `json:"queryType"`
	Error     string          `json:"error"`
}

type TableInfo struct {
	Name     string `json:"name"`
	ColCount int    `json:"colCount"`
}

var db *sql.DB

func queryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := QueryResponse{}
	rawQuery := strings.TrimSpace(req.Query)

	fmt.Printf("[%s] 📥 Running Query Block...\n", time.Now().Format("15:04:05"))

	cleanQuery := strings.ReplaceAll(rawQuery, "AUTO_INCREMENT", "AUTOINCREMENT")
	cleanQuery = strings.ReplaceAll(cleanQuery, "INT PRIMARY KEY AUTOINCREMENT", "INTEGER PRIMARY KEY AUTOINCREMENT")
	
	// Automatically convert standard INSERT to INSERT OR IGNORE so re-running never fails on duplicate keys
	cleanQuery = strings.ReplaceAll(cleanQuery, "INSERT INTO", "INSERT OR IGNORE INTO")
	cleanQuery = strings.ReplaceAll(cleanQuery, "insert into", "INSERT OR IGNORE INTO")

	queries := strings.Split(cleanQuery, ";")

	var lastSelectQuery string

	for _, q := range queries {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}

		upperQ := strings.ToUpper(q)

		if strings.HasPrefix(upperQ, "SELECT") {
			lastSelectQuery = q
		} else {
			_, execErr := db.Exec(q)
			if execErr != nil {
				// Ignore table already exists error
				if strings.Contains(execErr.Error(), "already exists") {
					fmt.Printf("[%s] ⚠️ Table already exists, continuing...\n", time.Now().Format("15:04:05"))
				} else {
					fmt.Printf("[%s] ❌ Error: %s\n", time.Now().Format("15:04:05"), execErr.Error())
					resp.Error = execErr.Error()
					json.NewEncoder(w).Encode(resp)
					return
				}
			}
		}
	}

	if lastSelectQuery != "" {
		resp.QueryType = "SELECT"
		rows, err := db.Query(lastSelectQuery)
		if err != nil {
			fmt.Printf("[%s] ❌ Select Error: %s\n", time.Now().Format("15:04:05"), err.Error())
			resp.Error = err.Error()
			json.NewEncoder(w).Encode(resp)
			return
		}
		defer rows.Close()

		cols, err := rows.Columns()
		if err != nil {
			resp.Error = err.Error()
			json.NewEncoder(w).Encode(resp)
			return
		}
		resp.Columns = cols

		for rows.Next() {
			columns := make([]interface{}, len(cols))
			columnPointers := make([]interface{}, len(cols))
			for i := range columns {
				columnPointers[i] = &columns[i]
			}

			if err := rows.Scan(columnPointers...); err != nil {
				resp.Error = err.Error()
				json.NewEncoder(w).Encode(resp)
				return
			}

			rowValues := make([]interface{}, len(cols))
			for i, col := range columns {
				if val, ok := col.([]byte); ok {
					rowValues[i] = string(val)
				} else {
					rowValues[i] = col
				}
			}
			resp.Rows = append(resp.Rows, rowValues)
		}
	} else {
		resp.QueryType = "EXEC"
		resp.Message = "✅ Execution completed successfully!"
	}

	fmt.Printf("[%s] ✅ Execution Finished Successfully.\n", time.Now().Format("15:04:05"))
	json.NewEncoder(w).Encode(resp)
}

func tablesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%';")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var name string
		rows.Scan(&name)

		colRows, err := db.Query(fmt.Sprintf("PRAGMA table_info('%s');", name))
		colCount := 0
		if err == nil {
			for colRows.Next() {
				colCount++
			}
			colRows.Close()
		}

		tables = append(tables, TableInfo{Name: name, ColCount: colCount})
	}

	json.NewEncoder(w).Encode(tables)
}

func main() {
	var err error
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	http.HandleFunc("/api/query", queryHandler)
	http.HandleFunc("/api/tables", tablesHandler)
	http.Handle("/", http.FileServer(http.Dir(".")))

	fmt.Println("🚀 Server running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}