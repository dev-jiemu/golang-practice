package main

import (
	"flag"
	"fmt"
	"golang.org/x/mod/sumdb/storage"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var dataFile string

type Chore struct {
	Complate    bool
	Description string
}

func init() {
	flag.StringVar(&dataFile, "file", "housework.db", "data file")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			`Usage : %s [flags] [add chore, ...|complate #]
				and add comma-separated chores
			complate complate designated chore
			Flags: `, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func load() ([]*Chore, error) {
	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		return make([]*Chore), nil
	}

	df, err := os.Open(dataFile)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := df.Close(); err != nil {
			fmt.Printf("closing data file : %v", err)
		}
	}()

	return storage.Load(df), nil
}

func flush(chores []*Chore) error {
	df, err := os.Create(dataFile)
	if err != nil {
		return err
	}

	defer func() {
		if err := df.Close(); err != nil {
			fmt.Printf("closing data file : %v", err)
		}
	}()

	return storage.Flush(df, chores)
}

func list() error {
	chores, err := load()
	if err != nil {
		return err
	}

	if len(chores) == 0 {
		fmt.Println("You`re all caught up")
		return nil
	}

	fmt.Println("#\t[X]\tDescription")
	for i, chore := range chores {
		c := " "
		if chore.Complate {
			c = "X"
		}
		fmt.Printf("%d\t[%s]\t%s\n", i+1, c, chore.Description)
	}

	return nil
}

func add(s string) error {
	chores, err := load()
	if err != nil {
		return err
	}

	for _, chore := range strings.Split(s, ",") {
		if desc := strings.TrimSpace(chore); desc != "" {
			chores = append(chores, &Chore{
				Description: desc,
			})
		}
	}
	return flush(chores)
}

func complate(s string) error {
	i, err := strconv.Atoi(s)
	if err != nil {
		return err
	}

	chores, err := load()
	if err != nil {
		return err
	}

	if i < 1 || i > len(chores) {
		return fmt.Errorf("chore %d not found", i)
	}

	chores[i-1].Complate = true

	return flush(chores)
}
