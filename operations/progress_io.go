package operations

import "io"

type progressReader struct {
	r     io.Reader
	total int64
	done  int64
	cb    ProgressCallback
}

func newProgressReader(r io.Reader, total int64, cb ProgressCallback) io.Reader {
	if cb == nil {
		return r
	}
	return &progressReader{r: r, total: total, cb: cb}
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 {
		r.done += int64(n)
		r.cb(r.done, r.total)
	}
	return n, err
}

type progressWriter struct {
	w     io.Writer
	total int64
	done  int64
	cb    ProgressCallback
}

func newProgressWriter(w io.Writer, total int64, cb ProgressCallback) io.Writer {
	if cb == nil {
		return w
	}
	return &progressWriter{w: w, total: total, cb: cb}
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	if n > 0 {
		w.done += int64(n)
		w.cb(w.done, w.total)
	}
	return n, err
}
