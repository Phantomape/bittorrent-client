package main

import (
	"fmt"
	"io/ioutil"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	data, err := ioutil.ReadFile("./bootstrap.dat.torrent")
	check(err)
	fmt.Print(string(data))
}
