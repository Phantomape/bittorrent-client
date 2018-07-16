package torrent

import (
	"crypto/sha1"
	"fmt"
	"net/url"
	"strings"
)

// Magnet :
type Magnet struct {
	InfoHashes []string
	Names      []string
	Trackers   [][]string
}

/*
 * Example bittorrent magnet link:
 * => magnet:?xt=urn:btih:bbb6db69965af769f664b6636e7914f8735141b3&dn=Ubuntu-12.04
 *    -desktop-i386.iso
 * xt: exact topic
 *  ~ urn: uniform resource name
 *	~ btih: bittorrent infohash
 * dn: display name(optional)
 * tr: address tracker(optional)
 */
func parseMagnet(s string) (Magnet, error) {
	u, err := url.Parse(s)
	if err != nil {
		return Magnet{}, err
	}

	xts, ok := u.Query()["xt"]
	if !ok {
		return Magnet{}, fmt.Errorf("Magnet URI missing the 'xt' argument: " + s)
	}
	infoHashes := make([]string, 0, len(xts))
	for _, xt := range xts {
		s := strings.Split(xt, "urn:btih:")
		if len(s) != 2 {
			return Magnet{}, fmt.Errorf("Magnet URI xt parameter missing the 'urn:btih:' prefix. Not a bittorent link?")
		}
		ih := s[1]
		if len(ih) != sha1.Size*2 {
			return Magnet{}, fmt.Errorf("Magnet URIcontains infohash with unexpected length. Wanted %d, got %d: %v", sha1.Size, len(ih), ih)
		}
		infoHashes = append(infoHashes, s[1])
	}

	var names []string
	n, ok := u.Query()["dn"]
	if ok {
		names = n
	}

	var trackers [][]string
	tr, ok := u.Query()["tr"]
	if ok {
		trackers = [][]string{tr}
	}
	fmt.Println("Trackers: ", trackers)

	return Magnet{InfoHashes: infoHashes, Names: names, Trackers: trackers}, nil
}
