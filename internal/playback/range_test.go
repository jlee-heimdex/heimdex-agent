package playback

import (
	"testing"
)

func TestParseRange(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		size      int64
		wantStart int64
		wantEnd   int64
		wantNil   bool
		wantErr   error
	}{
		{"empty header", "", 1000, 0, 0, true, nil},
		{"full range", "bytes=0-999", 1000, 0, 999, false, nil},
		{"partial start", "bytes=500-", 1000, 500, 999, false, nil},
		{"suffix range", "bytes=-500", 1000, 500, 999, false, nil},
		{"single byte", "bytes=0-0", 1000, 0, 0, false, nil},
		{"middle range", "bytes=100-199", 1000, 100, 199, false, nil},
		{"beyond size clamped", "bytes=0-2000", 1000, 0, 999, false, nil},
		{"suffix larger than file", "bytes=-2000", 500, 0, 499, false, nil},
		{"last byte", "bytes=999-", 1000, 999, 999, false, nil},
		{"multi range takes first", "bytes=0-99, 200-299", 1000, 0, 99, false, nil},

		{"unsatisfiable start", "bytes=1000-", 1000, 0, 0, false, ErrUnsatisfiable},
		{"unsatisfiable beyond", "bytes=1500-2000", 1000, 0, 0, false, ErrUnsatisfiable},
		{"invalid format no bytes", "invalid", 1000, 0, 0, false, ErrInvalidRange},
		{"wrong unit", "chars=0-100", 1000, 0, 0, false, ErrInvalidRange},
		{"invalid start", "bytes=abc-100", 1000, 0, 0, false, ErrInvalidRange},
		{"invalid end", "bytes=0-abc", 1000, 0, 0, false, ErrInvalidRange},
		{"negative suffix", "bytes=-0", 1000, 0, 0, false, ErrInvalidRange},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRange(tt.header, tt.size)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("ParseRange() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseRange() unexpected error: %v", err)
				return
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseRange() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Errorf("ParseRange() = nil, want non-nil")
				return
			}

			if got.Start != tt.wantStart || got.End != tt.wantEnd {
				t.Errorf("ParseRange() = {%d, %d}, want {%d, %d}", got.Start, got.End, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestRange_ContentLength(t *testing.T) {
	tests := []struct {
		start int64
		end   int64
		want  int64
	}{
		{0, 99, 100},
		{0, 0, 1},
		{500, 999, 500},
	}

	for _, tt := range tests {
		r := &Range{Start: tt.start, End: tt.end}
		if got := r.ContentLength(); got != tt.want {
			t.Errorf("ContentLength() = %d, want %d", got, tt.want)
		}
	}
}

func TestRange_ContentRange(t *testing.T) {
	tests := []struct {
		start int64
		end   int64
		total int64
		want  string
	}{
		{0, 99, 1000, "bytes 0-99/1000"},
		{500, 999, 1000, "bytes 500-999/1000"},
		{0, 0, 1, "bytes 0-0/1"},
	}

	for _, tt := range tests {
		r := &Range{Start: tt.start, End: tt.end}
		if got := r.ContentRange(tt.total); got != tt.want {
			t.Errorf("ContentRange() = %s, want %s", got, tt.want)
		}
	}
}
