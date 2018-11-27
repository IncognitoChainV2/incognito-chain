package common

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const HashSize = 32

const MaxHashStringSize = HashSize * 2

var ErrHashStrSize = fmt.Errorf("max hash string length is %v bytes", MaxHashStringSize)

type Hash [HashSize]byte

func (hash Hash) MarshalJSON() ([]byte, error) {
	hashString := hash.String()
	return json.Marshal(hashString)
}

func (hash *Hash) UnmarshalJSON(data []byte) error {
	hashString := ""
	_ = json.Unmarshal(data, &hashString)
	hash.Decode(hash, hashString)
	return nil
}

/*
String returns the Hash as the hexadecimal string of the byte-reversed
 hash.
*/
func (hash Hash) String() string {
	for i := 0; i < HashSize/2; i++ {
		hash[i], hash[HashSize-1-i] = hash[HashSize-1-i], hash[i]
	}
	return hex.EncodeToString(hash[:])
}

/*
CloneBytes returns a copy of the bytes which represent the hash as a byte
slice.
NOTE: It is generally cheaper to just slice the hash directly thereby reusing the same bytes rather than calling this method.
*/
func (hash *Hash) CloneBytes() []byte {
	newHash := make([]byte, HashSize)
	copy(newHash, hash[:])

	return newHash
}

/*
SetBytes sets the bytes which represent the hash.  An error is returned if the number of bytes passed in is not HashSize.
*/
func (hash *Hash) SetBytes(newHash []byte) error {
	nhlen := len(newHash)
	if nhlen != HashSize {
		return fmt.Errorf("invalid hash length of %v, want %v", nhlen,
			HashSize)
	}
	copy(hash[:], newHash)

	return nil
}

// BytesToHash sets b to hash If b is larger than len(h), b will be cropped from the left.
func (hash *Hash) BytesToHash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}

/*
IsEqual returns true if target is the same as hash.
*/
func (hash *Hash) IsEqual(target *Hash) bool {
	if &hash == nil && target == nil {
		return true
	}
	if &hash == nil || target == nil {
		return false
	}
	return hash.String() == target.String()
}

/*
NewHash returns a new Hash from a byte slice.  An error is returned if the number of bytes passed in is not HashSize.
*/
func (hash Hash) NewHash(newHash []byte) (*Hash, error) {
	err := hash.SetBytes(newHash)
	if err != nil {
		return nil, err
	}
	return &hash, err
}

/*
// NewHashFromStr creates a Hash from a hash string.  The string should be
// the hexadecimal string of a byte-reversed hash, but any missing characters
// result in zero padding at the end of the Hash.
*/
func (self Hash) NewHashFromStr(hash string) (*Hash, error) {
	err := self.Decode(&self, hash)
	if err != nil {
		return nil, err
	}
	return &self, nil
}

/*
// Decode decodes the byte-reversed hexadecimal string encoding of a Hash to a
// destination.
*/
func (self *Hash) Decode(dst *Hash, src string) error {
	// Return error if hash string is too long.
	if len(src) > MaxHashStringSize {
		return ErrHashStrSize
	}

	// Hex decoder expects the hash to be a multiple of two.  When not, pad
	// with a leading zero.
	var srcBytes []byte
	if len(src)%2 == 0 {
		srcBytes = []byte(src)
	} else {
		srcBytes = make([]byte, 1+len(src))
		srcBytes[0] = '0'
		copy(srcBytes[1:], src)
	}

	// Hex decode the source bytes to a temporary destination.
	var reversedHash Hash
	_, err := hex.Decode(reversedHash[HashSize-hex.DecodedLen(len(srcBytes)):], srcBytes)
	if err != nil {
		return err
	}

	// Reverse copy from the temporary hash to destination.  Because the
	// temporary was zeroed, the written result will be correctly padded.
	for i, b := range reversedHash[:HashSize/2] {
		dst[i], dst[HashSize-1-i] = reversedHash[HashSize-1-i], b
	}

	return nil
}

/* Compare compare two hash
// hash = hash2 : return 0
// hash > hash2 : return 1
// hash < hahs2 : return -1
// Todo: @0xakk0r0kamui
*/
func (hash *Hash) Compare(hash2 *Hash) int {
	return 0
}
