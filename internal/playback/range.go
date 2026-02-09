package playback

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrInvalidRange  = errors.New("invalid range format")
	ErrUnsatisfiable = errors.New("range not satisfiable")
)

type Range struct {
	Start int64
	End   int64
}

func (r Range) ContentLength() int64 {
	return r.End - r.Start + 1
}

func (r Range) ContentRange(total int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", r.Start, r.End, total)
}

func ParseRange(header string, size int64) (*Range, error) {
	if header == "" {
		return nil, nil
	}

	if !strings.HasPrefix(header, "bytes=") {
		return nil, ErrInvalidRange
	}

	rangeSpec := strings.TrimPrefix(header, "bytes=")

	if idx := strings.Index(rangeSpec, ","); idx != -1 {
		rangeSpec = strings.TrimSpace(rangeSpec[:idx])
	}

	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		return nil, ErrInvalidRange
	}

	var start, end int64
	var err error

	if parts[0] == "" {
		suffixLen, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffixLen <= 0 {
			return nil, ErrInvalidRange
		}
		start = size - suffixLen
		if start < 0 {
			start = 0
		}
		end = size - 1
	} else {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 {
			return nil, ErrInvalidRange
		}

		if parts[1] == "" {
			end = size - 1
		} else {
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return nil, ErrInvalidRange
			}
		}
	}

	if start > end || start >= size {
		return nil, ErrUnsatisfiable
	}

	if end >= size {
		end = size - 1
	}

	return &Range{Start: start, End: end}, nil
}
