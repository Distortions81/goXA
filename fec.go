package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/reedsolomon"
)

const fecMagic = "FEC1"

func encodeWithFEC(inPath, outPath string) error {
	data, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}
	enc, err := reedsolomon.New(fecDataShards, fecParityShards)
	if err != nil {
		return err
	}
	shards, err := enc.Split(data)
	if err != nil {
		return err
	}
	if err := enc.Encode(shards); err != nil {
		return err
	}

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	shardSize := uint32(len(shards[0]))
	if _, err := out.Write([]byte(fecMagic)); err != nil {
		return err
	}
	if err := binary.Write(out, binary.LittleEndian, uint8(fecDataShards)); err != nil {
		return err
	}
	if err := binary.Write(out, binary.LittleEndian, uint8(fecParityShards)); err != nil {
		return err
	}
	if err := binary.Write(out, binary.LittleEndian, shardSize); err != nil {
		return err
	}
	if err := binary.Write(out, binary.LittleEndian, uint64(len(data))); err != nil {
		return err
	}
	for _, s := range shards {
		if _, err := out.Write(s); err != nil {
			return err
		}
	}
	return nil
}

func decodeWithFEC(name string) (string, func(), error) {
	f, err := os.Open(name)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	hdr := make([]byte, 4)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return "", nil, err
	}
	if string(hdr) != fecMagic {
		return "", nil, fmt.Errorf("invalid FEC file")
	}
	var dataShards, parityShards uint8
	var shardSize uint32
	var dataSize uint64
	if err := binary.Read(f, binary.LittleEndian, &dataShards); err != nil {
		return "", nil, err
	}
	if err := binary.Read(f, binary.LittleEndian, &parityShards); err != nil {
		return "", nil, err
	}
	if err := binary.Read(f, binary.LittleEndian, &shardSize); err != nil {
		return "", nil, err
	}
	if err := binary.Read(f, binary.LittleEndian, &dataSize); err != nil {
		return "", nil, err
	}

	total := int(dataShards) + int(parityShards)
	shards := make([][]byte, total)
	for i := 0; i < total; i++ {
		buf := make([]byte, shardSize)
		if _, err := io.ReadFull(f, buf); err != nil {
			return "", nil, err
		}
		shards[i] = buf
	}

	enc, err := reedsolomon.New(int(dataShards), int(parityShards))
	if err != nil {
		return "", nil, err
	}
	ok, err := enc.Verify(shards)
	if err != nil {
		return "", nil, err
	}
	if !ok {
		if err := enc.Reconstruct(shards); err != nil {
			return "", nil, err
		}
	}

	tmp, err := os.CreateTemp("", "goxa_fec_dec_*")
	if err != nil {
		return "", nil, err
	}
	if err := enc.Join(tmp, shards, int(dataSize)); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", nil, err
	}
	tmp.Close()
	return tmp.Name(), func() { os.Remove(tmp.Name()) }, nil
}
