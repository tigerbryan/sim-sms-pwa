// qmi-dbcheck validates an SQLite database without starting a QMI service.
package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: qmi-dbcheck /path/to/qmi-web.db")
		os.Exit(2)
	}
	db, err := sql.Open("sqlite", "file:"+os.Args[1]+"?mode=ro")
	if err != nil {
		fmt.Fprintf(os.Stderr, "qmi-dbcheck: open: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	var result string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		fmt.Fprintf(os.Stderr, "qmi-dbcheck: integrity_check: %v\n", err)
		os.Exit(1)
	}
	if result != "ok" {
		fmt.Fprintf(os.Stderr, "qmi-dbcheck: integrity_check returned %q\n", result)
		os.Exit(1)
	}
	fmt.Println("qmi-dbcheck: integrity_check=ok")
}
