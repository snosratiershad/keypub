package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"keypub/internal/db/.gen/table"
	"log"

	. "github.com/go-jet/jet/v2/sqlite"

	_ "github.com/mattn/go-sqlite3"

	"keypub/internal/db/.gen/model"
)

func main() {
	// Open SQLite database
	// If the database doesn't exist, it will be created
	db, err := sql.Open("sqlite3", "/home/ubuntu/data/keysdb.sqlite3")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { db.Close(); fmt.Println("closed db") }()

	// Verify the connection
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Successfully connected to SQLite database!")

	stmt := SELECT(table.VerificationCodes.AllColumns).
		FROM(table.VerificationCodes)

	var vcs []model.VerificationCodes

	// Execute the query
	err = stmt.Query(db, &vcs)
	if err != nil {
		log.Fatal(err)
	}

	// Print results
	for _, vc := range vcs {
		json, _ := json.MarshalIndent(vc, "", "    ")
		fmt.Println(string(json))
	}
}
