package rlepluslazy

import (
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/xerrors"
)

const Version = 0

var (
	ErrWrongVersion = errors.New("invalid RLE+ version")
	ErrDecode       = fmt.Errorf("invalid encoding for RLE+ version %d", Version)
)

type RLE struct {
	buf       []byte
	validated bool
}

func FromBuf(buf []byte) (RLE, error) {
	rle := RLE{buf: buf}

	if len(buf) > 0 && buf[0]&3 != Version {
		return RLE{}, xerrors.Errorf("could not create RLE+ for a buffer: %w", ErrWrongVersion)
	}

	return rle, nil
}

// Bytes returns the encoded RLE.
//
// Do not modify.
func (rle *RLE) Bytes() []byte {
	return rle.buf
}

// Validate is a separate function to show up on profile for repeated decode evaluation
func (rle *RLE) Validate() error {
	if !rle.validated {
		return ValidateRLE(rle.buf)
	}
	return nil
}

func (rle *RLE) RunIterator() (RunIterator, error) {
	err := rle.Validate()
	if err != nil {
		return nil, xerrors.Errorf("validation failed: %w", err)
	}

	source, err := DecodeRLE(rle.buf)
	if err != nil {
		return nil, xerrors.Errorf("decoding RLE: %w", err)
	}

	return source, nil
}

func (rle *RLE) Count() (uint64, error) {
	it, err := rle.RunIterator()
	if err != nil {
		return 0, err
	}
	return Count(it)
}

type jsonRes struct {
	Count uint64
	RLE   []uint64
}

// Encoded as an array of run-lengths, always starting with zeroes (absent values)
// E.g.: The set {0, 1, 2, 8, 9} is the bitfield 1110000011, and would be marshalled as [0, 3, 5, 2]
func (rle *RLE) MarshalJSON() ([]byte, error) {
	r, err := rle.RunIterator()
	if err != nil {
		return nil, err
	}

	count, err := rle.Count()
	if err != nil {
		return nil, err
	}

	var ret = jsonRes{}
	if r.HasNext() {
		first, err := r.NextRun()
		if err != nil {
			return nil, err
		}
		if first.Val {
			ret.RLE = append(ret.RLE, 0)
		}
		ret.RLE = append(ret.RLE, first.Len)

		for r.HasNext() {
			next, err := r.NextRun()
			if err != nil {
				return nil, err
			}

			ret.RLE = append(ret.RLE, next.Len)
		}
	} else {
		ret.RLE = []uint64{0}
	}
	ret.Count = count

	return json.Marshal(ret)
}

func (rle *RLE) UnmarshalJSON(b []byte) error {
	var buf = jsonRes{}

	if err := json.Unmarshal(b, &buf); err != nil {
		return err
	}

	runs := []Run{}
	val := false
	for i, v := range buf.RLE {
		if v == 0 {
			if i != 0 {
				return xerrors.New("Cannot have a zero-length run except at start")
			}
		} else {
			runs = append(runs, Run{
				Val: val,
				Len: v,
			})
		}
		val = !val
	}
	enc, err := EncodeRuns(&RunSliceIterator{Runs: runs}, []byte{})
	if err != nil {
		return xerrors.Errorf("encoding runs: %w", err)
	}
	rle.buf = enc

	return nil
}
