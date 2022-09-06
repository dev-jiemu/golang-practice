package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
)

type User struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// https://echo.labstack.com/
// echo practice
func main() {
	e := echo.New()
	/*
		e.GET("/", func(c echo.Context) error {
			return c.String(http.StatusOK, "hello world")
		})
	*/
	e.GET("/users/:id", getUser)
	e.POST("/users", saveUser)

	//middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	//http server start
	e.Logger.Fatal(e.Start(":3000"))
}

func getUser(c echo.Context) error {
	id := c.Param("id")
	return c.String(http.StatusOK, id)
}

func saveUser(c echo.Context) error {
	u := new(User)
	if err := c.Bind(u); err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, u)
}
