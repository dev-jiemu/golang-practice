package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func toBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	var err error

	res, err := http.Get("https://cdntest.brocallee.com:27051/v2/cdn/IMG_CORP_REG/p2YUL")
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	var base64Encoding string
	mimeType := http.DetectContentType(bytes)

	switch mimeType {
	case "image/jpeg":
		base64Encoding += "data:image/jpeg;base64,"
	case "image/png":
		base64Encoding += "data:image/png;base64,"
	}

	base64Encoding += toBase64(bytes)

	html := "<html><body>"
	html += "<img src=\"" + base64Encoding + "\"/>"
	html += "</body></html>"

	f, err := os.Create("image.html")
	check(err)
	w := bufio.NewWriter(f)
	n, err := w.WriteString(html)
	check(err)
	fmt.Printf("wrote %d bytes\n", n)
	w.Flush()

	err = ioutil.WriteFile("image.html", []byte(html), 0644)
	check(err)
}
