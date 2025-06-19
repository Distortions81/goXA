package goxa

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/klauspost/reedsolomon"
)

const fecMagic = "GOXAFEC"

type fileOffsetWriter struct {
	f   *os.File
	off int64
}

func (w *fileOffsetWriter) Write(p []byte) (int, error) {
	n, err := w.f.WriteAt(p, w.off)
	w.off += int64(n)
	return n, err
}

func encodeWithFEC(inPath, outPath string) error {
	doLog(false, "FEC encoding archive")

	in, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	enc, err := reedsolomon.NewStream(fecDataShards, fecParityShards)
	if err != nil {
		return err
	}

	shardSize := uint32((info.Size() + int64(fecDataShards) - 1) / int64(fecDataShards))

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

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
	if err := binary.Write(out, binary.LittleEndian, uint64(info.Size())); err != nil {
		return err
	}

	headerLen, err := out.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	p, done, finished := progressTicker(&progressData{total: info.Size(), speedWindowSize: time.Second * 5})
	p.file.Store(inPath)

	dataW := make([]io.Writer, fecDataShards)
	for i := range dataW {
		sw := &fileOffsetWriter{f: out, off: headerLen + int64(i)*int64(shardSize)}
		dataW[i] = progressWriter{w: sw, p: p}
	}

	if err := enc.Split(in, dataW, info.Size()); err != nil {
		close(done)
		<-finished
		return err
	}

	dataR := make([]io.Reader, fecDataShards)
	for i := range dataR {
		dataR[i] = io.NewSectionReader(out, headerLen+int64(i)*int64(shardSize), int64(shardSize))
	}
	parityW := make([]io.Writer, fecParityShards)
	for i := range parityW {
		sw := &fileOffsetWriter{f: out, off: headerLen + int64(fecDataShards+i)*int64(shardSize)}
		parityW[i] = progressWriter{w: sw, p: p}
	}

	if err := enc.Encode(dataR, parityW); err != nil {
		close(done)
		<-finished
		return err
	}

	close(done)
	<-finished
	if !noFlush {
		if err := out.Sync(); err != nil {
			return err
		}
	}
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

	headerLen, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", nil, err
	}

	enc, err := reedsolomon.NewStream(int(dataShards), int(parityShards))
	if err != nil {
		return "", nil, err
	}

	total := int(dataShards) + int(parityShards)
	shards := make([]io.Reader, total)
	for i := 0; i < total; i++ {
		shards[i] = io.NewSectionReader(f, headerLen+int64(i)*int64(shardSize), int64(shardSize))
	}

	ok, err := enc.Verify(shards)
	if err != nil {
		return "", nil, err
	}
	if !ok {
		// Not much we can do here without knowing bad shards
	}

	tmp, err := os.CreateTemp("", "goxa_fec_dec_*")
	if err != nil {
		return "", nil, err
	}

	p, done, finished := progressTicker(&progressData{total: int64(dataSize), speedWindowSize: time.Second * 5})
	p.file.Store(name)

	// recreate readers since Verify consumed them
	dataR := make([]io.Reader, int(dataShards))
	for i := 0; i < int(dataShards); i++ {
		dataR[i] = io.NewSectionReader(f, headerLen+int64(i)*int64(shardSize), int64(shardSize))
	}

	if err := enc.Join(progressWriter{w: tmp, p: p}, dataR, int64(dataSize)); err != nil {
		close(done)
		<-finished
		tmp.Close()
		os.Remove(tmp.Name())
		return "", nil, err
	}
	if !noFlush {
		tmp.Sync()
	}
	tmp.Close()
	close(done)
	<-finished
	return tmp.Name(), func() { os.Remove(tmp.Name()) }, nil
}
