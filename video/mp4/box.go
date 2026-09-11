package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
)

// box is one parsed box header.
type box struct {
	typ        string
	size       uint64
	headerSize int64
	payloadOff int64
	payloadEnd int64
}

func readBoxHeader(r io.ReaderAt, off, total int64) (box, error) {
	var b box
	if off+8 > total {
		return b, fmt.Errorf("%w: header at %d", ErrTruncated, off)
	}
	hdr := make([]byte, 8)
	if _, err := r.ReadAt(hdr, off); err != nil {
		return b, fmt.Errorf("%w: header at %d: %v", ErrTruncated, off, err)
	}
	size32 := binary.BigEndian.Uint32(hdr[0:4])
	typ := string(hdr[4:8])
	size := uint64(size32)
	headerSize := int64(8)
	if size == 1 {
		if off+16 > total {
			return b, fmt.Errorf("%w: largesize at %d", ErrTruncated, off)
		}
		ext := make([]byte, 8)
		if _, err := r.ReadAt(ext, off+8); err != nil {
			return b, fmt.Errorf("%w: largesize at %d: %v", ErrTruncated, off, err)
		}
		size = binary.BigEndian.Uint64(ext)
		headerSize = 16
		if size < 16 {
			return b, fmt.Errorf("%w: largesize %d at %d", ErrBadBox, size, off)
		}
	} else if size == 0 {
		size = uint64(total - off)
		if size < 8 {
			return b, fmt.Errorf("%w: zero size at %d", ErrBadBox, off)
		}
	} else if size < 8 {
		return b, fmt.Errorf("%w: small size %d at %d", ErrBadBox, size, off)
	}
	if typ == "uuid" {
		headerSize += 16
		if size < uint64(headerSize) {
			return b, fmt.Errorf("%w: uuid box too small at %d", ErrBadBox, off)
		}
	}
	end := off + int64(size)
	if end < off || end > total {
		return b, fmt.Errorf("%w: box %s size %d at %d exceeds file %d", ErrTruncated, typ, size, off, total)
	}
	b.typ = typ
	b.size = size
	b.headerSize = headerSize
	b.payloadOff = off + headerSize
	b.payloadEnd = end
	return b, nil
}

func readBytes(r io.ReaderAt, off, end int64) ([]byte, error) {
	if end < off {
		return nil, fmt.Errorf("%w: invalid range", ErrBadBox)
	}
	n := end - off
	if n == 0 {
		return []byte{}, nil
	}
	if n > 256<<20 {
		return nil, fmt.Errorf("%w: box payload too large (%d)", ErrUnsupported, n)
	}
	buf := make([]byte, n)
	if _, err := r.ReadAt(buf, off); err != nil {
		return nil, fmt.Errorf("%w: payload at %d: %v", ErrTruncated, off, err)
	}
	return buf, nil
}

// cursor walks boxes inside one in-memory payload.
type cursor struct {
	buf []byte
	off int
}

func (c *cursor) done() bool { return c.off >= len(c.buf) }

func (c *cursor) next() (typ string, payload []byte, err error) {
	if len(c.buf)-c.off < 8 {
		return "", nil, fmt.Errorf("%w: child header", ErrTruncated)
	}
	size32 := binary.BigEndian.Uint32(c.buf[c.off:])
	typ = string(c.buf[c.off+4 : c.off+8])
	size := uint64(size32)
	header := 8
	if size == 1 {
		if len(c.buf)-c.off < 16 {
			return "", nil, fmt.Errorf("%w: child largesize", ErrTruncated)
		}
		size = binary.BigEndian.Uint64(c.buf[c.off+8:])
		header = 16
		if size < 16 {
			return "", nil, fmt.Errorf("%w: child largesize %d", ErrBadBox, size)
		}
	} else if size == 0 {
		size = uint64(len(c.buf) - c.off)
	} else if size < 8 {
		return "", nil, fmt.Errorf("%w: child size %d", ErrBadBox, size)
	}
	if typ == "uuid" {
		header += 16
		if size < uint64(header) {
			return "", nil, fmt.Errorf("%w: uuid child too small", ErrBadBox)
		}
	}
	if uint64(len(c.buf)-c.off) < size {
		return "", nil, fmt.Errorf("%w: child %s overruns parent", ErrTruncated, typ)
	}
	payload = c.buf[c.off+header : c.off+int(size)]
	c.off += int(size)
	return typ, payload, nil
}

func u16(b []byte, off int) (uint16, bool) {
	if off+2 > len(b) {
		return 0, false
	}
	return binary.BigEndian.Uint16(b[off:]), true
}

func u32(b []byte, off int) (uint32, bool) {
	if off+4 > len(b) {
		return 0, false
	}
	return binary.BigEndian.Uint32(b[off:]), true
}

func u64(b []byte, off int) (uint64, bool) {
	if off+8 > len(b) {
		return 0, false
	}
	return binary.BigEndian.Uint64(b[off:]), true
}

func fullbox(b []byte) (version byte, flags uint32, body []byte, ok bool) {
	if len(b) < 4 {
		return 0, 0, nil, false
	}
	return b[0], uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), b[4:], true
}

func ticksToMs(ticks int64, timescale uint32) int64 {
	if timescale == 0 {
		return 0
	}
	if ticks < 0 {
		ticks = 0
	}
	return ticks * 1000 / int64(timescale)
}
