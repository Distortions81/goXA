package main

import (
	"crypto/sha256"
	"hash"
	"hash/crc32"

	crc16 "github.com/sigurn/crc16"
	"github.com/zeebo/blake3"
	"github.com/zeebo/xxh3"
)

const (
	defaultChecksumType = sumCRC16
	defaultChecksumLen  = 2
)

func newHasher(t uint8) hash.Hash {
	switch t {
	case sumCRC32:
		return crc32.NewIEEE()
	case sumCRC16:
		table := crc16.MakeTable(crc16.CRC16_CCITT_FALSE)
		return crc16.New(table)
	case sumXXHash:
		return xxh3.New()
	case sumSHA256:
		return sha256.New()
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
	case sumCRC16:
		return 2
	case sumXXHash:
		return 8
	case sumSHA256:
		return 32
	case sumBlake3:
		fallthrough
	default:
		return 32
	}
}
