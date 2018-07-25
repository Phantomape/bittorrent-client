package protocol

import "testing"

func TestEncodeHaveMessage(t *testing.T) {
	actualBytes, err := Message{
		Type:  Have,
		Index: 42,
	}.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	actualString := string(actualBytes)
	expectedString := "\x00\x00\x00\x05\x04\x00\x00\x00\x2a"
	if actualString != expectedString {
		// wtf is #v, can't find v in type field
		t.Fatalf("expected %#v, got %#v", expectedString, actualString)
	}
}
