// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"errors"
	"fmt"
)

// gifHeaderLen is the length of a GIF signature and logical screen descriptor.
const gifHeaderLen = 13

// errGIFTruncated reports a GIF that ends before its trailer.
var errGIFTruncated = errors.New("the gif ends before its trailer")

// gifFrames returns how many image blocks a GIF declares, without decoding any.
func gifFrames(data []byte) (int, error) {
	pos, err := gifHeaderEnd(data)
	if err != nil {
		return 0, err
	}
	frames := 0
	for pos < len(data) {
		if data[pos] == 0x3B {
			return frames, nil
		}
		if data[pos] == 0x2C {
			frames++
			if frames > 1 {
				return frames, nil
			}
		}
		pos, err = skipGIFBlock(data, pos)
		if err != nil {
			return 0, err
		}
	}
	return 0, errGIFTruncated
}

// gifHeaderEnd returns where the blocks after the header and global palette start.
func gifHeaderEnd(data []byte) (int, error) {
	if len(data) < gifHeaderLen {
		return 0, errGIFTruncated
	}
	if signature := string(data[:6]); signature != "GIF87a" && signature != "GIF89a" {
		return 0, fmt.Errorf("%q is not a gif signature", signature)
	}
	return gifHeaderLen + colorTableLen(data[10]), nil
}

// colorTableLen returns the bytes a color table flagged in flags occupies.
func colorTableLen(flags byte) int {
	if flags&0x80 == 0 {
		return 0
	}
	return 3 * (2 << (flags & 0x07))
}

// skipGIFBlock returns the position after the block starting at pos.
func skipGIFBlock(data []byte, pos int) (int, error) {
	switch data[pos] {
	case 0x21:
		return skipGIFSubBlocks(data, pos+2)
	case 0x2C:
		if len(data) < pos+11 {
			return 0, errGIFTruncated
		}
		return skipGIFSubBlocks(data, pos+10+colorTableLen(data[pos+9])+1)
	default:
		return 0, fmt.Errorf("unknown gif block 0x%02x", data[pos])
	}
}

// skipGIFSubBlocks returns the position after a length prefixed sub block chain.
func skipGIFSubBlocks(data []byte, pos int) (int, error) {
	for {
		if pos >= len(data) {
			return 0, errGIFTruncated
		}
		size := int(data[pos])
		pos++
		if size == 0 {
			return pos, nil
		}
		pos += size
	}
}
