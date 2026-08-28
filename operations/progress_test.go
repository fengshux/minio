package operations

import (
	"bytes"
	"io"
	"testing"
)

func TestProgressReaderReportsBytes(t *testing.T) {
	data := []byte("hello world")
	var calls [][2]int64

	r := newProgressReader(bytes.NewReader(data), int64(len(data)), func(done, total int64) {
		calls = append(calls, [2]int64{done, total})
	})

	buf := make([]byte, 4)
	for {
		_, err := r.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
	}

	if len(calls) < 3 {
		t.Fatalf("回调次数过少: %d", len(calls))
	}
	if calls[0] != [2]int64{4, 11} {
		t.Fatalf("首个回调 = %v, want [4 11]", calls[0])
	}
	if calls[len(calls)-1] != [2]int64{11, 11} {
		t.Fatalf("最后回调 = %v, want [11 11]", calls[len(calls)-1])
	}
}

func TestProgressWriterReportsBytes(t *testing.T) {
	var calls [][2]int64
	var dst bytes.Buffer

	w := newProgressWriter(&dst, 10, func(done, total int64) {
		calls = append(calls, [2]int64{done, total})
	})

	if _, err := w.Write([]byte("hello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := w.Write([]byte("world")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if got := dst.String(); got != "helloworld" {
		t.Fatalf("输出 = %q, want %q", got, "helloworld")
	}
	if len(calls) != 2 {
		t.Fatalf("回调次数 = %d, want 2", len(calls))
	}
	if calls[0] != [2]int64{5, 10} {
		t.Fatalf("首个回调 = %v, want [5 10]", calls[0])
	}
	if calls[1] != [2]int64{10, 10} {
		t.Fatalf("最后回调 = %v, want [10 10]", calls[1])
	}
}
