package Remon

import "time"

func createObserver() chan func() {
	ch := make(chan func())
	go observerLoop(ch)
	return ch
}

func observerLoop(ch chan func()) {
	for {
		select {
		case fn := <-ch:
			{
				fn()
			}
		}
	}
}

func addFunc(ch chan func(), fn func()) {
	ch <- fn
}

func test1() {
	ch := createObserver()
	addFunc(ch, func() {
		println("asfs")
	})
	addFunc(ch, func() {
		println("asfs")
	})
	time.Sleep(10 * time.Second)
}
