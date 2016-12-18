package main

import (
	"fmt"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

type Database struct {
	conn *sql.DB
}

func (d *Database) ConnectionString() string {
	return SQL_USER + ":" + SQL_PASS + "@tcp(" + SQL_HOST + ":" + SQL_PORT + ")/" + SQL_DB
}

func (d *Database) Open() bool {
	conn := d.ConnectionString();
	fmt.Println("Connecting to: ", conn)
	db, err := sql.Open("mysql", conn)
	if err != nil {
		fmt.Println(err.Error())
	}
	d.conn = db
	d.conn.SetMaxOpenConns(MAX_CONNECTIONS)

	// Check that we can ping the DB box as the connection is lazy loaded when we fire the query
	err = d.conn.Ping()
	if err != nil {
		fmt.Println(err.Error())
	}

	return true
}

// Given a query string and a list of variadic parameters bindings this
// method will
func (d *Database) Query(query string, parameters ...interface{}) (*sql.Rows, error) {
	if d.conn == nil {
		fmt.Println("Spawning a new connection")
		d.Open()
	}

	LogInDebugMode("Interfaces: ", parameters)

	LogInDebugMode("Preparing query: " + query)
	stmt, err := d.conn.Prepare(query)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	defer stmt.Close()

	if len(parameters) > 0 {
		rows, err := stmt.Query(parameters...)
		if err != nil {
			fmt.Println("Error sending query: ", err.Error())
			return nil, err
		}
		return rows, nil
	} else {
		rows, err := stmt.Query()
		if err != nil {
			fmt.Println("Error sending query: ", err.Error())
			return nil, err
		}
		return rows, nil
	}
}

func (d *Database) Insert(query string, parameters ...interface{}) (int64, error) {
	tx, err := d.conn.Begin()
	if err != nil {
		fmt.Println("Error creating transaction: ", err.Error())
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(query)
	if err != nil {
		fmt.Println("Error preparing insert query: ", err)
	}

	res, err := stmt.Exec(parameters...)
	if err != nil {
		fmt.Println("Exec err when inserting: ", err.Error())
	} else {
		id, err := res.LastInsertId()
		if err != nil {
			fmt.Println("Error when fetching last insert id: ", err.Error())
		} else {
			LogInDebugMode("returning iD: ", id)
			err = tx.Commit()
			if err != nil {
				panic(err.Error())
			}
			stmt.Close()
			return id, nil
		}
	}

	err = tx.Commit()
	if err != nil {
		panic(err.Error())
	}
	stmt.Close()
	return -1, err
}

func (d *Database) Close() {
	if d.conn != nil {
		fmt.Println("Closing DB connection")
		err := d.conn.Close()
		if err == nil {
			fmt.Println("DB connection disposed successfully")
		} else {
			fmt.Println("Failed to close DB connection: ", err)
		}
	} else {
		fmt.Println("DB Connection was already closed")
	}
}

func (d *Database) CloseRows(rows *sql.Rows) {
	if err := rows.Close(); err != nil {
		fmt.Println("Close error: ", err)
	}
}