package main

import (
	"fmt"
	"strconv"
	"strings"
)

// scanInput simple parse input value from stdin
type scanInput struct {
	Placeholder string

	// Vaild custom vaildation call back
	Vaild func(s *string) (bool, error)

	// Options simple options
	Options []*inputOptions

	// Vaildate input length
	Minlength int
	Default   string

	// Password hide vlaue
	Password bool
}

// inputOptions support map line-number, and value's first character to value
type inputOptions struct {
	Label string
	Value string
}

// scan parse input based on scanInput
func (i scanInput) scan() string {
	var res string
	var prompt []string
	fmt.Println(i.Placeholder)
	if i.Default != "" {
		if i.Password && !showSecret {
			prompt = append(prompt, "\033[1m******\033[0m")
		} else {
			prompt = append(prompt, "\033[1m"+i.Default+"\033[0m")
		}
	}
	for j := 0; j < len(i.Options); j++ {
		fmt.Printf("%d  %-20s %s\n", j+1, i.Options[j].Value, i.Options[j].Label)
		if i.Options[j].Value != i.Default {
			prompt = append(prompt, i.Options[j].Value)
		}
	}

	for {
		fmt.Printf("[ %s ] : ", strings.Join(prompt, "/"))
		fmt.Scanln(&res)
		if res == "" && i.Default != "" {
			return i.Default
		}
		included := false
		for j := 0; j < len(i.Options); j++ {
			if i.Options[j].Value == res || i.Options[j].Value[:1] == res {
				included = true
				res = i.Options[j].Value
			}
		}
		if len(i.Options) > 0 && !included {
			intValue, e := strconv.ParseInt(res, 10, 0)
			if e != nil || intValue < 0 || len(i.Options) < int(intValue) {
				fmt.Println("input value un-acceptable")
				continue
			}
			return i.Options[intValue-1].Value
		}
		if len(res) < i.Minlength {
			fmt.Printf("input length should been greater then %d\n", i.Minlength)
			continue
		}
		if i.Vaild != nil {
			if b, e := i.Vaild(&res); !b && e != nil {
				fmt.Printf("%s\n", e.Error())
				continue
			}
		}
		return res
	}
}
