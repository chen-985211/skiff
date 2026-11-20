package aria2

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"slices"
)

type byteSpan struct{ start, end int64 } // end is exclusive
var errCheckpoint = errors.New("无法读取有效的 aria2 下载检查点")

// aria2 DefaultBtProgressInfoFile v1 stores completed pieces and 16 KiB
// blocks of unfinished pieces, most significant bit first. The apparent file
// size (including sparse holes) is not a measure of which bytes will be reused.
func checkpointSpans(path string) (int64, []byteSpan, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, nil, err
	}
	defer f.Close()
	const maxControl = 16 << 20
	data, err := io.ReadAll(io.LimitReader(f, maxControl+1))
	if err != nil {
		return 0, nil, err
	}
	if len(data) > maxControl {
		return 0, nil, errCheckpoint
	}
	r := bytes.NewReader(data)
	var h struct {
		Version                     uint16
		Extensions, HashSize, Piece uint32
		Total, Upload               uint64
	}
	if err := binary.Read(r, binary.BigEndian, &h); err != nil || h.Version != 1 || h.Extensions != 0 || h.HashSize != 0 || h.Piece == 0 || h.Total == 0 || h.Total > math.MaxInt64 {
		return 0, nil, errCheckpoint
	}
	total, piece := int64(h.Total), int64(h.Piece)
	pieces := 1 + (total-1)/piece
	var spans []byteSpan
	readBits := func(base, length, unit int64) bool {
		bits := 1 + (length-1)/unit
		var size uint32
		if binary.Read(r, binary.BigEndian, &size) != nil || int64(size) != (bits+7)/8 || uint64(size) > uint64(r.Len()) {
			return false
		}
		for i := int64(0); i < int64(size); i++ {
			v, _ := r.ReadByte()
			if v == 0 {
				continue
			}
			for j := int64(0); j < 8; j++ {
				if v&(0x80>>j) == 0 {
					continue
				}
				bit := 8*i + j
				if bit >= bits {
					return false
				}
				offset := bit * unit
				start := base + offset
				end := start + min(unit, length-offset)
				if n := len(spans); n > 0 && spans[n-1].end == start {
					spans[n-1].end = end
				} else {
					if len(spans) >= 1<<20 {
						return false
					}
					spans = append(spans, byteSpan{start, end})
				}
			}
		}
		return true
	}
	if !readBits(0, total, piece) {
		return 0, nil, errCheckpoint
	}
	var count uint32
	if binary.Read(r, binary.BigEndian, &count) != nil || int64(count) > pieces {
		return 0, nil, errCheckpoint
	}
	for range count {
		var part struct{ Index, Length uint32 }
		if binary.Read(r, binary.BigEndian, &part) != nil || int64(part.Index) >= pieces {
			return 0, nil, errCheckpoint
		}
		start := int64(part.Index) * piece
		if int64(part.Length) != min(piece, total-start) || !readBits(start, int64(part.Length), 16<<10) {
			return 0, nil, errCheckpoint
		}
	}
	if r.Len() != 0 {
		return 0, nil, errCheckpoint
	}
	slices.SortFunc(spans, func(a, b byteSpan) int {
		if a.start < b.start {
			return -1
		}
		if a.start > b.start {
			return 1
		}
		return 0
	})
	merged := spans[:0]
	for _, span := range spans {
		n := len(merged)
		if n > 0 && merged[n-1].end >= span.start {
			merged[n-1].end = max(merged[n-1].end, span.end)
		} else {
			merged = append(merged, span)
		}
	}
	return total, merged, nil
}
