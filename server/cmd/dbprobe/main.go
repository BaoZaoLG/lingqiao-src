package main

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "C:/Users/Li/AppData/Local/Temp/paritydemo/data/lingqiao.db")
	rows, err := db.Query(`SELECT time, typeof(time) FROM audit_log ORDER BY id DESC LIMIT 3`)
	if err != nil { panic(err) }
	defer rows.Close()
	for rows.Next() {
		var raw, kind string
		_ = rows.Scan(&raw, &kind)
		fmt.Printf("audit_log.time raw=%q typeof=%s\n", raw, kind)
	}
	r2, _ := db.Query(`SELECT created_at, typeof(created_at) FROM cards LIMIT 2`)
	defer r2.Close()
	for r2.Next() {
		var raw, kind string
		_ = r2.Scan(&raw, &kind)
		fmt.Printf("cards.created_at raw=%q typeof=%s\n", raw, kind)
	}
}
