package db

//go:generate sh -c "rm ./keysdb.sqlite3; exit 0"
//go:generate sqlite3 -init ./schema.sql ./keysdb.sqlite3
//go:generate go run github.com/go-jet/jet/v2/cmd/jet -source=sqlite -schema=main -path=$PWD/.gen -dsn=$PWD/keysdb.sqlite3
