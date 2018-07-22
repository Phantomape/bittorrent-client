package metainfo

// Info : the info dictionary
type Info struct {
	Name        string // suggested name for the file, purely advisory
	Pieces      []byte // length is a multiple of 20
	PieceLength int64  // number of bytes
	Length      int64  // length of the file
	Files       []FileInfo
}
