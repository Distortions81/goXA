package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/klauspost/reedsolomon"
)

const fecMagic = "GOXAFEC"

func encodeWithFEC(inPath, outPath string) error {
	doLog(false, "FEC encoding archive")
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

	p, done, finished := progressTicker(&progressData{total: int64(len(data)), speedWindowSize: time.Second * 5})
	p.file.Store(inPath)
	w := progressWriter{w: out, p: p}
	for _, s := range shards {
		if _, err := w.Write(s); err != nil {
			close(done)
			<-finished
			return err
		}
	}
	close(done)
	<-finished
	return nil
}

func decodeWithFEC(name string) (string, func(), error) {
	doLog(false, "FEC decoding archive")
	f, err := os.Open(name)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	hdr := make([]byte, len(fecMagic))
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

	p, done, finished := progressTicker(&progressData{total: int64(dataSize), speedWindowSize: time.Second * 5})
	p.file.Store(name)

	if err := enc.Join(progressWriter{w: tmp, p: p}, shards, int(dataSize)); err != nil {
		close(done)
		<-finished
		tmp.Close()
		os.Remove(tmp.Name())
		return "", nil, err
	}
	tmp.Close()
	close(done)
	<-finished
	return tmp.Name(), func() { os.Remove(tmp.Name()) }, nil
}
