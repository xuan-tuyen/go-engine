package skiplist

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"
)

func Test001(t *testing.T) {
	s := NewInt32Map()

	d := make([]int32, 100000)
	for i := 0; i < len(d); i++ {
		d[i] = int32(i)
	}
	rand.Shuffle(len(d), func(i, j int) {
		d[i], d[j] = d[j], d[i]
	})

	for _, dd := range d {
		s.Set(dd, strconv.Itoa(int(dd)))
	}

	for i := 0; i < len(d); i++ {
		firstValue, ok := s.Get(int32(i))
		if !ok {
			fmt.Println(firstValue)
		}
	}

	s.Delete(int32(7))

	for i := 0; i < len(d); i++ {
		_, ok := s.Get(int32(i))
		if !ok {
			fmt.Println(i)
		}
	}

	s.Set(int32(7), "niner")

	secondValue, ok := s.Get(int32(7))
	if ok {
		fmt.Println(secondValue)
	}

	// Iterate through all the elements, in order.
	unboundIterator := s.Iterator()
	sum := 0
	for unboundIterator.Next() {
		sum += int(unboundIterator.Key().(int32))
	}
	fmt.Printf("%d\n", sum)

	boundIterator := s.Range(int32(32544), int32(32546))
	// Iterate only through elements in some range.
	for boundIterator.Next() {
		fmt.Printf("%d: %s\n", boundIterator.Key(), boundIterator.Value())
	}

}
