package db

//go:generate sh -c "rm ./keysdb.sqlite3; exit 0"
//go:generate sqlite3 -init ./schema.sql ./keysdb.sqlite3
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate
