package wallet

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"errors"

	"github.com/ninjadotorg/cash-prototype/cashec"
	"github.com/ninjadotorg/cash-prototype/common/base58"
)

const (
	// FirstHardenedChild is the index of the firxt "harded" Child Key as per the
	// bip32 spec
	//FirstHardenedChild = uint32(0x80000000)

	// PublicKeyCompressedLength is the byte count of a compressed public Key
	PublicKeyCompressedLength = 33
)

const (
	PriKeyType      = byte(0x0)
	PubKeyType      = byte(0x1)
	ReadonlyKeyType = byte(0x2)
)

var (
	// PrivateWalletVersion is the version flag for serialized private keys
	//PrivateWalletVersion, _ = hex.DecodeString("0488ADE4")

	// PublicWalletVersion is the version flag for serialized private keys
	//PublicWalletVersion, _ = hex.DecodeString("0488B21E")

	// ErrSerializedKeyWrongSize is returned when trying to deserialize a Key that
	// has an incorrect length
	ErrSerializedKeyWrongSize = errors.New("Serialized keys should by exactly 82 bytes")

	// ErrHardnedChildPublicKey is returned when trying to create a harded Child
	// of the public Key
	ErrHardnedChildPublicKey = errors.New("Can't create hardened Child for public Key")

	// ErrInvalidChecksum is returned when deserializing a Key with an incorrect
	// checksum
	ErrInvalidChecksum = errors.New("Checksum doesn't match")

	// ErrInvalidPrivateKey is returned when a derived private Key is invalid
	ErrInvalidPrivateKey = errors.New("Invalid private Key")

	// ErrInvalidPublicKey is returned when a derived public Key is invalid
	ErrInvalidPublicKey = errors.New("Invalid public Key")
)

// KeySet represents a bip32 extended Key
type Key struct {
	Depth       byte   // 1 bytes
	ChildNumber []byte // 4 bytes
	ChainCode   []byte // 32 bytes
	KeyPair     cashec.KeySet
}

// NewMasterKey creates a new master extended Key from a Seed
func NewMasterKey(seed []byte) (*Key, error) {
	// Generate Key and chaincode
	hmac := hmac.New(sha512.New, []byte("Bitcoin Seed"))
	_, err := hmac.Write(seed)
	if err != nil {
		Logger.log.Error(err)
		return nil, err
	}
	intermediary := hmac.Sum(nil)

	// Split it into our Key and chain code
	keyBytes := intermediary[:32]  // use to create master private/public keypair
	chainCode := intermediary[32:] // be used with public Key (in keypair) for new Child keys

	// Validate Key
	/*err = validatePrivateKey(keyBytes)
	if err != nil {
		return nil, err
	}*/

	keyPair := (&cashec.KeySet{}).GenerateKey(keyBytes)

	// Create the Key struct
	key := &Key{
		ChainCode:   chainCode,
		KeyPair:     *keyPair,
		Depth:       0x00,
		ChildNumber: []byte{0x00, 0x00, 0x00, 0x00},
	}

	return key, nil
}

// NewChildKey derives a Child Key from a given parent as outlined by bip32
func (key *Key) NewChildKey(childIdx uint32) (*Key, error) {
	intermediary, err := key.getIntermediary(childIdx)
	if err != nil {
		return nil, err
	}

	newSeed := []byte{}
	newSeed = append(newSeed[:], intermediary[:32]...)
	newKeypair := (&cashec.KeySet{}).GenerateKey(newSeed)
	// Create Child KeySet with data common to all both scenarios
	childKey := &Key{
		ChildNumber: uint32Bytes(childIdx),
		ChainCode:   intermediary[32:],
		Depth:       key.Depth + 1,
		KeyPair:     *newKeypair,
	}

	return childKey, nil
}

func (key *Key) getIntermediary(childIdx uint32) ([]byte, error) {
	// Get intermediary to create Key and chaincode from
	// Hardened children are based on the private Key
	// NonHardened children are based on the public Key
	childIndexBytes := uint32Bytes(childIdx)

	var data []byte
	//if childIdx >= FirstHardenedChild {
	//	data = append([]byte{0x0}, Key.KeySet.PrivateKey...)
	//} else {
	// data = key.KeySet.PublicKey
	//}
	data = append(data, childIndexBytes...)

	hmac := hmac.New(sha512.New, key.ChainCode)
	_, err := hmac.Write(data)
	if err != nil {
		return nil, err
	}
	return hmac.Sum(nil), nil
}

// Serialize a KeySet to a 78 byte byte slice
func (key *Key) Serialize(keyType byte) ([]byte, error) {
	// Write fields to buffer in order
	buffer := new(bytes.Buffer)
	buffer.WriteByte(keyType)
	if keyType == PriKeyType {

		buffer.WriteByte(key.Depth)
		buffer.Write(key.ChildNumber)
		buffer.Write(key.ChainCode)

		// Private keys should be prepended with a single null byte
		keyBytes := make([]byte, 0)
		keyBytes = append(keyBytes, byte(len(key.KeyPair.PrivateKey))) // set length
		keyBytes = append(keyBytes, key.KeyPair.PrivateKey[:]...)      // set pri-key
		buffer.Write(keyBytes)
	} else if keyType == PubKeyType {
		keyBytes := make([]byte, 0)
		keyBytes = append(keyBytes, byte(len(key.KeyPair.PublicKey.Apk))) // set length Apk
		keyBytes = append(keyBytes, key.KeyPair.PublicKey.Apk[:]...)      // set Apk

		keyBytes = append(keyBytes, byte(len(key.KeyPair.PublicKey.Pkenc))) // set length Pkenc
		keyBytes = append(keyBytes, key.KeyPair.PublicKey.Pkenc[:]...)      // set Pkenc
		buffer.Write(keyBytes)
	} else if keyType == ReadonlyKeyType {
		keyBytes := make([]byte, 0)
		keyBytes = append(keyBytes, byte(len(key.KeyPair.ReadonlyKey.Apk))) // set length Apk
		keyBytes = append(keyBytes, key.KeyPair.ReadonlyKey.Apk[:]...)      // set Apk

		keyBytes = append(keyBytes, byte(len(key.KeyPair.ReadonlyKey.Skenc))) // set length Skenc
		keyBytes = append(keyBytes, key.KeyPair.ReadonlyKey.Skenc[:]...)      // set Pkenc
		buffer.Write(keyBytes)
	}

	// Append the standard doublesha256 checksum
	serializedKey, err := addChecksumToBytes(buffer.Bytes())
	if err != nil {
		Logger.log.Error(err)
		return nil, err
	}

	return serializedKey, nil
}

// Base58CheckSerialize encodes the KeySet in the standard Bitcoin base58 encoding
func (key *Key) Base58CheckSerialize(keyType byte) string {
	serializedKey, err := key.Serialize(keyType)
	if err != nil {
		return ""
	}

	return base58.Base58Check{}.Encode(serializedKey, byte(0x00))
}

// Deserialize a byte slice into a KeySet
func Deserialize(data []byte) (*Key, error) {
	//if len(data) != 101 {
	//	return nil, ErrSerializedKeyWrongSize
	//}
	var key = &Key{}
	keyType := data[0]
	if keyType == PriKeyType {
		key.Depth = data[1]
		key.ChildNumber = data[2:6]
		key.ChainCode = data[6:38]
		keyLength := int(data[38])

		copy(key.KeyPair.PrivateKey[:], data[39:39+keyLength])
	} else if keyType == PubKeyType {
		apkKeyLength := int(data[1])
		copy(key.KeyPair.PublicKey.Apk[:], data[2:2+apkKeyLength])
		pkencKeyLength := int(data[apkKeyLength+2])
		copy(key.KeyPair.PublicKey.Pkenc[:], data[3+apkKeyLength: 3+apkKeyLength+pkencKeyLength])
	} else if keyType == ReadonlyKeyType {
		apkKeyLength := int(data[1])
		copy(key.KeyPair.ReadonlyKey.Apk[:], data[2:2+apkKeyLength])
		skencKeyLength := int(data[apkKeyLength+2])
		copy(key.KeyPair.ReadonlyKey.Skenc[:], data[3+apkKeyLength: 3+apkKeyLength+skencKeyLength])
	}

	// validate checksum
	cs1 := base58.ChecksumFirst4Bytes(data[0: len(data)-4])
	cs2 := data[len(data)-4:]
	for i := range cs1 {
		if cs1[i] != cs2[i] {
			return nil, ErrInvalidChecksum
		}
	}
	return key, nil
}

// Base58CheckDeserialize deserializes a KeySet encoded in base58 encoding
func Base58CheckDeserialize(data string) (*Key, error) {
	b, _, err := base58.Base58Check{}.Decode(data)
	if err != nil {
		return nil, err
	}
	return Deserialize(b)
}
