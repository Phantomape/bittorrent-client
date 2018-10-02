After we've built our tracer, we can move on to the design and implementation of peer. Peers communicate with each other directly by opening TCP connections.

#   Peer Protocol
Let's take a look at what peer protocol is:
>   The peer protocol refers to pieces of the file by index as described in the metainfo file, starting at zero. When a peer finishes downloading a piece and checks that the hash matches, it announces that it has that piece to all of its peers. 

Peer protocol operates over TCP or [uTP](http://www.bittorrent.org/beps/bep_0029.html), and the connection between two peers are symmetrical, which means messages sent in both directions look the same. The first message received will be a handshake message, followed by a never-ending stream of messages.

#   Peer
Peer is an abstraction in the communication, which would open TCP connections to send and receive messages. In the OOP-paradigm, it is reasonable to model it as an object.

#   Messages

##  Handshake Message
I just copy from the official doc:
>   The handshake starts with character ninteen (decimal) followed by the string 'BitTorrent protocol'. The leading character is a length prefix, put there in the hope that other new protocols may do the same and thus be trivially distinguishable from each other.

Before we implement the handshake message, let's take a look at the other implementation. The [anacrolix's handshake implementation](https://github.com/anacrolix/torrent/blob/master/peer_protocol/handshake.go) is kinda obscure because of its arguments. It seems that the ```sock``` in the argument is what we read from and write to. It feels weird at first sight but it is actually correct because handshake means to establish connection and socket is just a abstraction layer that provides read/write functionality.

In the implementation, it will first start a new thread to handle the writes to the socket. Then it will read from the socket to verify whether data is the same, cause data can flow in either direction.

```
// Handshake : transfer handshake message
func Handshake(sock io.ReadWriter, ih *metainfo.Hash, peerID [20]byte, extensions PeerExtensionBits) (res HandshakeResult, ok bool, err error) {
	writeDone := make(chan error, 1) // error value sent when the writer completes
	postCh := make(chan []byte, 4)   // 4 means buffer capacity ???
	go handshakeWriter(sock, postCh, writeDone)

	defer func() {
		// Under what occasion will it return not ok without error ?
		close(postCh)
		if !ok {
			return
		}
		if err != nil {
			panic(err)
		}

		err = <-writeDone
		if err != nil {
			err = fmt.Errorf("error writing: %s", err)
		}
	}()

	post := func(bb []byte) {
		select {
		case postCh <- bb:
		default:
			panic("mustn't block while posting")
		}
	}

	// Start handshake
	post([]byte(Header))
	post(extensions[:])
	if ih != nil {
		post(ih[:])
		post(peerID[:])
	}

	// Receive acknowledge signal
	var b [68]byte
	_, err = io.ReadFull(sock, b[:68])
	if err != nil {
		err = nil // What ?
		return
	}
	if string(b[:20]) != Header {
		return
	}
	missinggo.CopyExact(&res.PeerExtensionBits, b[20:28])
	missinggo.CopyExact(&res.Hash, b[28:48])
	missinggo.CopyExact(&res.PeerID, b[48:68])

	// Don't know what it is, BEP003 didn't specify this part
	if ih == nil {
		post(res.Hash[:])
		post(peerID[:])
	}

	ok = true
	return
}
```

[Github commit: add handshake](https://github.com/Phantomape/bittorrent-client/commit/2a8ebcdac447ab3d7eeb267b8b34ed5909243d33)

##  Stream Messages
Messages of length zero are keepalive messages and they will be ignored. Other messages are called non-keepalive messages. All no-keepalive message start with a single byte which gives their type based on the [doc](http://www.bittorrent.org/beps/bep_0003.html). So it would be reasonable to construct an enum to represent the message type:
```
type MessageType byte
const (
	Choke         MessageType = 0
    ...
)
```

[Github commit: peer message type](https://github.com/Phantomape/bittorrent-client/commit/e6ba02a466e66df6328d61851cef807e4af13721)

The next thing we're gonna do is to implement the type ```Message``` used in the protocol. According to BEP003, different message type have different payload and [anacrolix's implementation](https://github.com/anacrolix/torrent/blob/master/peer_protocol/msg.go) solve this abstraction by stuffing everything into the type. In my opinion, this way is faster but less organized, another approach is to let the ```Message``` have a payload string and a decode type, which brings overhead to decoding the peer message but more readability to the code. I'll try to follow anacrolix's implementation in terms of the design.

The first four message: ```Choke```, ```Unchoke```, ```Interested```, ```NotInterested``` have no payload, so let's take a look at the rest of messages.

The payload of ```Bitfield``` message is a bitfield, each bit represents an index. If the index is sent by downloader, it will be set to 1.

The payload of ```Have``` message is a single number: the index of which that downloader just completed and checked the hash of.

The payload of ```Request``` message contains an index, begin and length.

The ```Cancel``` message has the same payload as the ```Request``` message, but it is sent at the end of a download, during what's called 'endgame mode'.

```Piece``` message contains an index, begin and piece.
```
type Message struct {
    Keepalive               bool
    Type                    MessageType
    // These three attributes are very common in the message
    Index, Begin, Length    uint32
    Piece                   []byte
    Bitfield                []bool
}
```

#	Implementation
I'm not quite satisfied with [anacrolix's implementation](https://github.com/anacrolix/torrent/tree/master/peer_protocol). In his implementation, a type ```Decoder``` exists to convert stream of bytes to the type ```Message```. However, the encoding from ```Message``` to bytes is done by calling ```MarshalBinary``` method in [connection.go](https://github.com/anacrolix/torrent/blob/master/connection.go). Right now, in order to have a workable version of the client, we'll use a similar implementation like anacrolix, but will change it to using an ```Encoder``` type in the future.

