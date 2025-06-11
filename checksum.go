package main

import (
	"hash"
	"hash/crc32"

	"github.com/zeebo/blake3"
	"github.com/zeebo/xxh3"
)

const (
	defaultChecksumType = sumBlake3
	defaultChecksumLen  = 32
)

func newHasher(t uint8) hash.Hash {
	switch t {
	case sumCRC32:
		return crc32.NewIEEE()
	case sumXXHash:
		return xxh3.New()
	case sumBlake3:
		fallthrough
	default:
		return blake3.New()
	}
}

func checksumLen(t uint8) int {
	switch t {
	case sumCRC32:
		return 4
	case sumXXHash:
		return 8
	case sumBlake3:
		fallthrough
	default:
		return 32
	}
}
