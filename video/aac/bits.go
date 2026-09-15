package aac

import (
	"fmt"
	"sync"
)

// bitReaderMSB reads AAC payloads MSB-first. It peers GetBitContext
// (get_bits.h) for the operations landing 2 needs; the struct layout
// and VLC tables are our own.
type bitReaderMSB struct {
	b   []byte
	pos int
	n   int
}

func newBitReaderMSB(b []byte) *bitReaderMSB {
	return &bitReaderMSB{b: b, n: len(b) * 8}
}

func (r *bitReaderMSB) left() int { return r.n - r.pos }

func (r *bitReaderMSB) show(n int) (int, error) {
	if n <= 0 || n > 24 {
		return 0, fmt.Errorf("%w: show %d", ErrBadADTS, n)
	}
	if r.left() < n {
		return 0, fmt.Errorf("%w: need %d have %d", ErrTruncated, n, r.left())
	}
	v := 0
	for i := 0; i < n; i++ {
		p := r.pos + i
		v = (v << 1) | int((r.b[p>>3]>>(7-(p&7)))&1)
	}
	return v, nil
}

func (r *bitReaderMSB) read(n int) (int, error) {
	v, err := r.show(n)
	if err != nil {
		return 0, err
	}
	r.pos += n
	return v, nil
}

func (r *bitReaderMSB) read1() (int, error) { return r.read(1) }

func (r *bitReaderMSB) skip(n int) error {
	if r.left() < n {
		return fmt.Errorf("%w: skip %d have %d", ErrTruncated, n, r.left())
	}
	r.pos += n
	return nil
}

// huffTree is a binary prefix tree built from (code,bits) pairs.
type huffTree struct {
	// child0/child1 use -1 for absent; leaf holds symbol or -1.
	child0 []int32
	child1 []int32
	leaf   []int32
}

func buildTree(codes []uint32, bits []uint8) (*huffTree, error) {
	t := &huffTree{child0: []int32{-1}, child1: []int32{-1}, leaf: []int32{-1}}
	for sym := range codes {
		code := codes[sym]
		nb := int(bits[sym])
		if nb <= 0 || nb > 24 {
			return nil, fmt.Errorf("bad huffman bits %d", nb)
		}
		node := 0
		for i := nb - 1; i >= 0; i-- {
			bit := (code >> uint(i)) & 1
			var next int32
			if bit == 0 {
				next = t.child0[node]
			} else {
				next = t.child1[node]
			}
			if next < 0 {
				next = int32(len(t.leaf))
				t.child0 = append(t.child0, -1)
				t.child1 = append(t.child1, -1)
				t.leaf = append(t.leaf, -1)
				if bit == 0 {
					t.child0[node] = next
				} else {
					t.child1[node] = next
				}
			}
			node = int(next)
		}
		if t.leaf[node] >= 0 {
			return nil, fmt.Errorf("duplicate huffman code")
		}
		t.leaf[node] = int32(sym)
	}
	return t, nil
}

func (t *huffTree) decode(r *bitReaderMSB) (int, error) {
	node := 0
	for {
		if node < 0 || node >= len(t.leaf) {
			return 0, fmt.Errorf("%w: huffman walk", ErrBadADTS)
		}
		if t.leaf[node] >= 0 {
			// Leaf reached only after consuming its bits; but root
			// with zero bits is invalid. Require at least one step.
			return int(t.leaf[node]), nil
		}
		b, err := r.read1()
		if err != nil {
			return 0, err
		}
		if b == 0 {
			node = int(t.child0[node])
		} else {
			node = int(t.child1[node])
		}
		if node < 0 {
			return 0, fmt.Errorf("%w: huffman code", ErrBadADTS)
		}
	}
}

var (
	huffOnce sync.Once
	huffErr  error
	sfTree   *huffTree
	bookTree [11]*huffTree
)

func ensureHuff() error {
	huffOnce.Do(func() {
		var err error
		sfCodes32 := make([]uint32, len(sfCodes))
		for i, v := range sfCodes {
			sfCodes32[i] = v
		}
		sfBits8 := make([]uint8, len(sfBits))
		copy(sfBits8, sfBits[:])
		if sfTree, err = buildTree(sfCodes32, sfBits8); err != nil {
			huffErr = err
			return
		}
		for i := 0; i < 11; i++ {
			var codes []uint32
			var bits []uint8
			switch i {
			case 0:
				codes = u16to32(bookCodes1[:])
				bits = u8copy(bookBits1[:])
			case 1:
				codes = u16to32(bookCodes2[:])
				bits = u8copy(bookBits2[:])
			case 2:
				codes = u16to32(bookCodes3[:])
				bits = u8copy(bookBits3[:])
			case 3:
				codes = u16to32(bookCodes4[:])
				bits = u8copy(bookBits4[:])
			case 4:
				codes = u16to32(bookCodes5[:])
				bits = u8copy(bookBits5[:])
			case 5:
				codes = u16to32(bookCodes6[:])
				bits = u8copy(bookBits6[:])
			case 6:
				codes = u16to32(bookCodes7[:])
				bits = u8copy(bookBits7[:])
			case 7:
				codes = u16to32(bookCodes8[:])
				bits = u8copy(bookBits8[:])
			case 8:
				codes = u16to32(bookCodes9[:])
				bits = u8copy(bookBits9[:])
			case 9:
				codes = u16to32(bookCodes10[:])
				bits = u8copy(bookBits10[:])
			case 10:
				codes = u16to32(bookCodes11[:])
				bits = u8copy(bookBits11[:])
			}
			var t *huffTree
			if t, err = buildTree(codes, bits); err != nil {
				huffErr = err
				return
			}
			bookTree[i] = t
		}
	})
	return huffErr
}

func u16to32(in []uint16) []uint32 {
	out := make([]uint32, len(in))
	for i, v := range in {
		out[i] = uint32(v)
	}
	return out
}

func u8copy(in []uint8) []uint8 {
	out := make([]uint8, len(in))
	copy(out, in)
	return out
}
