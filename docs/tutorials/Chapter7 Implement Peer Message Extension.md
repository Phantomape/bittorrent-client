From this chapter, we're gonna dive into some existing BEPs to further power our client. The first feature we're gonna implement is the [Fast Extension](http://www.bittorrent.org/beps/bep_0006.html)

#   Modification to Reserved Bits
The fast extension is enabled by setting the third least significant bit of the last reserved byte in the handshake:
```
reserved[7] |= 0x04
```

#   Modification to Stream Messages
Apart from the handshake, the stream of messages also need modification. The ```Request```, ```Choke```, ```Unchoke``` and ```Cancel``` messages need to be modified, and some message types are added.

[Github commit: new message types for fast extension]()

#   New Message Types
*   ```HaveAll```: message sender has all of the pieces
*   ```HaveNone```: message sender has none of the pieces
*   ```SuggestPiece```: provide the index of suggest piece
*   ```RejectRequest```: similar to ```Request``` message, with index, begin and length