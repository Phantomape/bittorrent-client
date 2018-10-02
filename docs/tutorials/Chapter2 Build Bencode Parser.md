This article will walk through the process of Bencode parsing and help you to build a bencode parser from scratch. First, we need an example torrent to verify our code. These two sample torrents are placed under ```test/data/``` folder.

#   Read Torrent File

[Github commit: read torrent file](https://github.com/Phantomape/bittorrent-client/commit/b35022a6267c184d0d8e64b9b906d2b91ebc3ee8)

Let's try to open the torrent first, after all, it is just a file :D.
```
data, err := ioutil.ReadFile("./bootstrap.dat.torrent")
fmt.Print(string(data))
```
The ```ioutil.ReadFile``` reads the file into byte slice, from which we can see sth. like this:
```
d8:announce35:udp://tracker.openbittorrent...
```

The output, unfortunately, is not readable by human because the file is encoded using Bencode. So the first step of the whole tutorial is to build a bencode parser. Let's begin :D.

#   API Design

In order to build a good package with readable code, a good API design is essential, I take a look at severl implementation of bencode parser, which are [zeebo's bencode parser](https://github.com/zeebo/bencode), [anacrolix's bencode parser](https://github.com/anacrolix/torrent) and [jackpal's bencode parser](https://github.com/jackpal/bencode-go). All of them trys to shadow the API design of golang's JSON encoding/decoding package. So the two main APIs are:
```
func Marshal(v interface{}) ([]byte, error)
func Unmarshal(data [] byte, v interface{}) error
```

Personally speaking, I like anacrolix's api design better cause the code is well-organized and its API is similar to the golang's JSON package. So I'll try to make comparison with its implementation as we build our own parser from scratch.

It seems weird to use ```Unmarshal``` and ```Marshal``` as two main API names instead of using ```Encode``` and ```Decode```, which is the traditional way of API naming. However, ```Encode``` and ```Decode``` function also exist among these APIs. The main difference between them is that ```Encode``` is designed to support the common operation of reading and writing streams of JSON data.