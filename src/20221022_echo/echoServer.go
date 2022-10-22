package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
)

/*
/employee GET
/employee/{id} GET
/employee POST
/employee/{id} PUT
/employee DELETE
*/

const (
	host     = "localhost"
	database = "practice"
	user     = "root"
	password = ""
)

type Employee struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	Salary string `json:"salary""`
	Age    string `json:"age"`
}

type Employees struct {
	Employees []Employee `json:"employees"`
}

func main() {

	var err error
	// Initialize connection string.
	var connectionString = fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?allowNativePasswords=true", user, password, host, database)

	// Initialize connection object.
	db, err := sql.Open("mysql", connectionString)
	if err != nil {
		log.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		panic(err)
	} else {
		fmt.Println("DB Conneted..")
	}

	e := echo.New()

	//link setting
	e.GET("/employee", func(c echo.Context) error {
		var err error
		sqlStatement := "SELECT ID, NAME, SALARY, AGE FROM TB_EMPLOYEES ORDER BY ID"

		rows, err := db.Query(sqlStatement)
		if err != nil {
			fmt.Println(err)
		}
		defer rows.Close()
		result := Employees{}

		for rows.Next() {
			employee := Employee{}
			err := rows.Scan(&employee.Id, &employee.Name, &employee.Salary, &employee.Age)
			if err != nil {
				return err
			}

			result.Employees = append(result.Employees, employee)
		}

		return c.JSON(http.StatusCreated, result)
	})
}
