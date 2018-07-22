package metainfo

import (
	"io"
	"os"

	"github.com/anacrolix/torrent/bencode"
)

// MetaInfo : data structure with the parsed data from torrent
type MetaInfo struct {
	Announce  string
	InfoBytes bencode.Bytes
}

// Load : load the metainfo from an io.Reader
func Load(r io.Reader) (*MetaInfo, error) {
	var mi MetaInfo
	d := bencode.NewDecoder(r)
	err := d.Decode(&mi)
	if err != nil {
		return nil, err
	}
	return &mi, nil
}

// LoadFromFile : load the metainfo from a file
func LoadFromFile(filename string) (*MetaInfo, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// UnmarshalInfo : decode the metainfo
func (mi MetaInfo) UnmarshalInfo() (info Info, err error) {
	err = bencode.Unmarshal(mi.InfoBytes, &info)
	return
}
