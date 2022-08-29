package main

import "io/ioutil"

/*
func main() {

	fi, err := os.Open("D:\\sample.txt")
	if err != nil {
		panic(err)
	}
	defer fi.Close()

	fo, err := os.Create("D:\\sample.txt")
	if err != nil {
		panic(err)
	}
	defer fo.Close()

	buff := make([]byte, 1024)

	for {
		//파일 읽기
		cnt, err := fi.Read(buff)
		if err != nil && err != io.EOF {
			panic(err)
		}

		//다읽었으면 for문 종료
		if cnt == 0 {
			break
		}

		//그대로 쓰기
		_, err = fo.Write(buff[:cnt])
		if err != nil {
			panic(err)
		}

	}

}
*/

func main() {
	bytes, err := ioutil.ReadFile("D:\\sample.txt")
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile("D:\\sample.txt", bytes, 0)
	if err != nil {
		panic(err)
	}

}
