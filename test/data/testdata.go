package main

import (
	"fmt"
	"io/ioutil"

	bencode "github.com/anacrolix/torrent/bencode"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func loadFile(name string) []byte {
	data, err := ioutil.ReadFile(name)
	check(err)
	return data
}

func main() {
	data := loadFile("./bootstrap.dat.torrent")
	var v interface{}
	bencode.Unmarshal(data, &v)
	fmt.Println(v)
}
