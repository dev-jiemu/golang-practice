package main

import "fmt"

type Point struct {
	X, Y int
}

// Value receiver
// Point struct 에서 사용하는 '메서드' (not function)
func (p Point) printInfo() {
	fmt.Println("value receiver")
	fmt.Println(p)
	fmt.Println(p.X)
	fmt.Println(p.Y)
}

func (p *Point) add(a int) {
	p.X += a
	p.Y += a
}

func (p Point) mul(a int) {
	p.X *= a
	p.Y *= a
}

func main() {

	p := Point{1, 3}
	p.printInfo()

	fmt.Println("call by value, call by reference")
	p2 := Point{3, 4}
	p2.add(10)  //13, 14       - call by value
	p2.mul(100) //1300, 1400   - call by reference
	fmt.Println(p2)

}
