package bencode

const (
	scanSkipSpace = iota
	scanError
)

// SyntaxError is a description of a BENCODE syntax error.
type SyntaxError struct {
	msg    string
	Offset int64
}

func (e *SyntaxError) Error() string {
	return e.msg
}

// scanner like the scanner in encoding/json
type scanner struct {
	step  func(*scanner, byte) int
	err   error
	bytes int64 // total bytes consumed
}

func (s *scanner) reset() {
	s.step = nil
	s.err = nil
}

func (s *scanner) eof() int {
	if s.err != nil {
		return scanError
	}
	s.step(s, ' ')
	if s.err == nil {
		s.err = &SyntaxError{"unexpected end of BENCOD input", s.bytes}
	}
	return scanError
}

func checkValid(data []byte, scan *scanner) error {
	return nil
}
