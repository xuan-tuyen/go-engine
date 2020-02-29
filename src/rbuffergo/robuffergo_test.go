package rbuffergo

import (
	"fmt"
	"testing"
)

func TestNew1(t *testing.T) {
	rob := NewROBuffer(10, 1, 20)
	rob.Set(1, 1)
	rob.Set(1, 1)
	err := rob.Set(10, 10)
	if err != nil {
		fmt.Println(err)
	}
	err = rob.Set(11, 11)
	if err != nil {
		fmt.Println(err)
	}
	err = rob.Set(13, 13)
	if err != nil {
		fmt.Println(err)
	}
	err = rob.Set(19, 19)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(rob.size)
	for e := rob.FrontInter(); e != nil; e = e.Next() {
		fmt.Println(e.Value)
	}

	err, d := rob.Front()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(d)
	rob.PopFront()

	err, d = rob.Front()
	if err != nil {
		fmt.Println(err)
	}

	rob.Set(2, 2)
	rob.Set(3, 3)
	rob.Set(4, 4)
	rob.Set(5, 5)
	rob.Set(6, 6)
	rob.Set(7, 7)
	rob.Set(8, 8)
	rob.Set(9, 9)
	err = rob.Set(10, 10)
	if err != nil {
		fmt.Println(err)
	}
	err = rob.Set(11, 11)
	if err != nil {
		fmt.Println(err)
	}
	err = rob.Set(12, 12)
	if err != nil {
		fmt.Println(err)
	}

	err, d = rob.Front()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(d)
	rob.PopFront()
	err = rob.Set(12, 12)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(rob.size)
	fmt.Println("---")
	for i := 0; i < 100; i++ {
		err, d := rob.Front()
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(d)
		rob.PopFront()
		err = rob.Set((13+i)%20, (13+i)%20)
		if err != nil {
			fmt.Println(err)
		}
	}
	fmt.Println("---")
	for e := rob.FrontInter(); e != nil; e = e.Next() {
		fmt.Println(e.Value)
	}
	fmt.Println(rob.Full())
}

func TestNew2(t *testing.T) {
	rob := NewROBuffer(10, 1, 20)
	rob.Set(1, 1)
	err, d := rob.Get(1)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(d)
	err, d = rob.Get(11)
	if err != nil {
		fmt.Println(err)
	}
	err, d = rob.Get(2)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(d)
	fmt.Println(rob.Full())
}

func TestNew3(t *testing.T) {
	rob := NewROBuffer(5, 0, 10)
	rob.Set(0, 0)
	rob.Set(1, 1)
	rob.Set(2, 2)
	rob.Set(3, 3)
	rob.Set(4, 4)
	fmt.Println(rob.Full())
	err := rob.Set(5, 5)
	fmt.Println(err)
}

func TestNew4(t *testing.T) {
	rob := NewROBuffer(5, 0, 10)
	id := 0
	index := 0
	for i := 0; i < 100; i++ {
		for rob.Size() < 5 {
			err := rob.Set(id, index)
			index++
			if err != nil {
				fmt.Println(err)
			}
			id++
			if id >= 10 {
				id = 0
			}
		}

		for !rob.Empty() {
			err, value := rob.Front()
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println(value)
			err = rob.PopFront()
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}
