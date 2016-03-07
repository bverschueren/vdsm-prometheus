package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func TestReadJson(t *testing.T) {
	file, err := os.Open("get_all_vm_stats.json")
	check(err)
	bytes, err := ioutil.ReadAll(bufio.NewReader(file))
	check(err)
	fmt.Print(string(bytes[:]))
}
