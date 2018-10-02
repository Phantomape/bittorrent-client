After successfully building our own parser, we could move on to the next part: tracker. If you don't want to build the bencode parser, I strongly recommend you to use [anacrolix's bencode parser](https://github.com/anacrolix/torrent) to unblock the learning process. Also, before we dive into this chapter, take a look at the [bittorrent protocol](http://www.morehawes.co.uk/the-bittorrent-protocol) first.

[Github commit: use existing package to decode torrent](https://github.com/Phantomape/bittorrent-client/commit/8f9fc16703bb74513ab58ff2fa88f8907837b310)

#   Announcing
Announce is the term referring to the URL of the trackernormal HTTP request sent to the tracker. The tracker also supports another form of request called scraping, which I'll covered in the next section.

##   Request
A tracker is essentially a HTTP server with one single endpoint, which will provide a list of peers that we can connect. To fetch such information, we need to first sent the HTTP request, which contains several parameters listed below:
*   ```info_hash```: a 20 byte sha1 hash
*   ```peer_id```: a 20 byte string to identify peers and ourselves(we are actually peers as well).
*   ```port```: the port number listening on
*   ```uploaded```: number of bytes we've sent to other peers(is the same with all peers?)
*   ```downloaded```: number of bytes downloaded(need verified?)
*   ```left```: number of bytes left to download
*   ```event```: the current state of the client, only 3 possible values: started, paused and stopped.
*   ```compact```: indicating whether to return a compact peer list
*   ```ip```: (optional) IP address of the client machine, in dotted format.
*   ```numwant```: (optional) number of peers the client wishes to receive from the tracker.
*   ```key```: (optional) allows a client to identify itself if their IP address changes. Saw this in the implementation but could not find it from bep003.
*   ```trackerid```: (optional) if previous announce contained a tracker id, it should be set here.

##   Response
According to the [protocol](http://www.bittorrent.org/beps/bep_0003.html), the response is a bencoded object, with the following fields:
*   ```interval```: frequency the client should request an updated peer list from the tracker
*   ```min interval```: the lower bound of ```interval```
*   ```peers```: a list of dictionaries including: peer id, IP and ports of all the peers
*   ```failure message```: if present, then no other keys are included. The value is a human readable error message as to why the
request failed
*   ```warning message```: similar to failure message, but response still gets processed
*   ```tracker id```: a string that the client should send back with its next announce

#   Scraping
A scrape is another kind of request that queries the state of a given torrent that is managed by the tracker. 

##  Request
According to the definition in [bittorrent.org](http://www.bittorrent.org/beps/bep_0048.html), we must first determine the endpoint for our request, which is done by locating the string "announce" in the path section of the announce URL and replacing it with the string "scrape". A scrape request contains no HTTP body, but instead uses HTTP query parameters to encode its key-value pairs. There is only one key, the info_hash key specified in BEP 003. This key can be appended multiple times with different values for collecting data about multiple swarms in one request. An example is shown below:
```
http://tracker/scrape?info_hash=xxxxxxxxxxxxxxxxxxxx&info_hash=yyyyyyyyyyyyyyyyyyyy
```

##  Response
The response to a successful request is a bencoded dictionary containing one key-value pair: the key ```files``` with the value being a dictionary of the 20-byte string representation of an infohash paired with a dictionary of swarm metadata. The fields found in the swarm metadata dictionary are as follows:
*   ```complete```: the number of active peers that have completed downloading
*   ```incomplete```: the number of active peers that have not completed downloading
*   ```downloaded```: the number of peers that have ever completed downloading

The response to an unsuccessful request is a bencoded dictionary with the key failure_reason and a string value as described in BEP 003. An example of a successful response is shown below:
```
{
  'files': {
    'xxxxxxxxxxxxxxxxxxxx': {'complete': 11, 'downloaded': 13772, 'incomplete': 19},
    'yyyyyyyyyyyyyyyyyyyy': {'complete': 21, 'downloaded': 206, 'incomplete': 20}
  }
}
```

#   Testing
To test the functionality of the tracker, we can use curl or whatever package that could fire a HTTP request.

#   Implementation
We could first try to implement the necessary data structure first. In order to make things easier for me, I'll use the OOP paradigm to do it. First, we define the data structure related to the announce.
```
type AnnounceRequest struct {
    InfoHash    [20]byte
    PeerId      [20]byte
    Downloaded  int64
    Uploaded    int64
    Left        int64
    Event       AnnounceEvent
    IP          uint32
    Port        uint16
    Key         int32
    NumWant     int32
}
...
```

[Github commit: declare necessary data type](https://github.com/Phantomape/bittorrent-client/commit/852513be28139944bd37717fa7e81745a8ab016c)

This piece of code should be easy to understand as I'm just treating request/response as an object.

The next thing we should do is to think about what we're gonna do(LOL). Because we don't actually implement the tracker, so the object that sends the request is not tracker but us. However, we haven't implemented the client yet, so we don't have a subject to take on the action of sending request. We need another layer of abstraction here before we move on to the implementation of the client. In [anacrolix's implementation](https://github.com/anacrolix/torrent/blob/d950677f67c26793d1c551266dfc56f81081b48a/tracker/tracker.go), only a ```Do``` function is defined and this function will be used to handle a request and respond, which is quite convenient. The subject for this function is an object called ```Announce```, which would be the layer of abstraction we want. (Maybe I'd move the definition of ```Do``` into the client like the ```httpClient.Do``` [here](https://golang.org/pkg/net/http/#Client.Do)).

There're two kinds of tracker protocol that we need to implement, but right now we only focus the one using HTTP, I'll come back later for the [UDP tracker protocol](http://www.bittorrent.org/beps/bep_0015.html)

[Github commit: add announce for abstraction](https://github.com/Phantomape/bittorrent-client/commit/8e0721bbe00a8547fa078575c71fcee858d5452e)

The function that process HTTP announce is ```announceHTTP(a Announce, url *url.URL)```, which takes in the ```Announce``` object and a url pointer that points to tracker URL and return an ```AnnounceResponse```. The first step we need to do is to set relevant parameters in the request, which is done using a function called ```setAnnounceParams```. After the response has been received, we would copy it to a buffer and convert it into the type ```httpResponse``` and then we extract the necessary information into an ```AnnounceResponse```(I don't like this step, cause it doesn't look that symmetrical to me since we don't have a ```httpRequest``` type).

[Github commit: add logic for handling http announce](https://github.com/Phantomape/bittorrent-client/commit/78487774c1e28858767992bea2a80af6aa1baa01)
