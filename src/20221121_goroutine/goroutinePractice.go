package main

import (
	"fmt"
	"sync"
	"time"
)

type Account struct {
	Balance int
}

func DepositAndWithdraw(account *Account) {
	fmt.Println("Balance: ", account.Balance)
	if account.Balance > 0 {
		panic(fmt.Sprintf("Balance should not be negative value: %d", account.Balance))
	}
	account.Balance += 1000
	time.Sleep(time.Millisecond)
	account.Balance -= 1000
}

// 순차적으로 실행하지 않고 무작위로 실행할 경우 panic 발생
// (여러 고루틴이 동일한 메모리 주소의 값을 변경하고 있어서 동시성 문제 발생)
func main() {
	var wg sync.WaitGroup

	account := &Account{Balance: 10}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			for {
				DepositAndWithdraw(account)
			}
		}()
	}
	wg.Wait()
}
