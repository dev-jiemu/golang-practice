package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"time"
)

// https://soyoung-new-challenge.tistory.com/126
func main() {
	db, err := sql.Open("mysql", "root:root@/shop")
	defer db.Close()

	if err != nil {
		panic(err)
	}

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)

	//fmt.Println("connection success", db)

	var name string

	err = db.QueryRow("Select user_id from tb_user").Scan(&name)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("user_id", name)

}
