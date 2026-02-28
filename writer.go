package trace_file

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type rotatingWriter struct {
	mutex sync.Mutex

	file     *os.File
	buffer   *bufio.Writer
	filename string

	size     int64
	lines    int64
	maxSize  int64
	maxLine  int64
	slice    string
	compress bool
	maxAge   time.Duration

	maxFiles int

	startTime time.Time
}

func newRotatingWriter(filename string, maxSize int64, slice string, maxLine int64, compress bool, maxAge time.Duration, maxFiles int) (*rotatingWriter, error) {
	w := &rotatingWriter{
		filename: filename,
		maxSize:  maxSize,
		maxLine:  maxLine,
		slice:    slice,
		compress: compress,
		maxAge:   maxAge,
		maxFiles: maxFiles,
	}
	if err := w.open(); err != nil {
		return nil, err
	}
	_ = w.cleanup()
	return w, nil
}

func (w *rotatingWriter) open() error {
	if err := os.MkdirAll(filepath.Dir(w.filename), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(w.filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}

	w.file = file
	w.buffer = bufio.NewWriterSize(file, 64*1024)
	w.size = info.Size()
	w.startTime = time.Now()
	return nil
}

func (w *rotatingWriter) Close() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	var err error
	if w.buffer != nil {
		if e := w.buffer.Flush(); e != nil {
			err = e
		}
	}
	if w.file != nil {
		if e := w.file.Close(); e != nil && err == nil {
			err = e
		}
		w.file = nil
	}
	return err
}

func (w *rotatingWriter) WriteLine(line string) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	lineBytes := int64(len(line) + 1)

	if w.shouldRotate(lineBytes) {
		if err := w.rotate(); err != nil {
			return err
		}
	}

	if _, err := w.buffer.WriteString(line); err != nil {
		return err
	}
	if err := w.buffer.WriteByte('\n'); err != nil {
		return err
	}

	w.size += lineBytes
	w.lines++

	return w.buffer.Flush()
}

func (w *rotatingWriter) shouldRotate(incoming int64) bool {
	if w.file == nil {
		return true
	}
	if w.maxSize > 0 && (w.size+incoming) > w.maxSize {
		return true
	}
	if w.maxLine > 0 && w.lines >= w.maxLine {
		return true
	}
	if w.slice != "" && !sameSliceWindow(w.slice, w.startTime, time.Now()) {
		return true
	}
	return false
}

func (w *rotatingWriter) rotate() error {
	if w.buffer != nil {
		if err := w.buffer.Flush(); err != nil {
			return err
		}
	}
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
	}

	if _, err := os.Stat(w.filename); err == nil {
		rotated := rotatedName(w.filename, time.Now())
		if err := os.Rename(w.filename, rotated); err != nil {
			return err
		}
		if w.compress {
			go compressRotatedFile(rotated)
		}
	}

	w.file = nil
	w.buffer = nil
	w.size = 0
	w.lines = 0
	w.startTime = time.Now()

	if err := w.cleanup(); err != nil {
		return err
	}

	return w.open()
}

func compressRotatedFile(path string) {
	src, err := os.Open(path)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "trace-file compress open failed: %v\n", err)
		return
	}
	defer src.Close()

	dstPath := path + ".gz"
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "trace-file compress create failed: %v\n", err)
		return
	}

	gw := gzip.NewWriter(dst)
	_, copyErr := io.Copy(gw, src)
	closeGzipErr := gw.Close()
	closeDstErr := dst.Close()

	if copyErr != nil || closeGzipErr != nil || closeDstErr != nil {
		_ = os.Remove(dstPath)
		if copyErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "trace-file compress copy failed: %v\n", copyErr)
		}
		if closeGzipErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "trace-file compress finalize failed: %v\n", closeGzipErr)
		}
		if closeDstErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "trace-file compress close failed: %v\n", closeDstErr)
		}
		return
	}

	if err := os.Remove(path); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "trace-file compress cleanup failed: %v\n", err)
	}
}

func (w *rotatingWriter) cleanup() error {
	if w.maxAge <= 0 && w.maxFiles <= 0 {
		return nil
	}

	files, err := listRotatedFiles(w.filename)
	if err != nil {
		return err
	}

	now := time.Now()
	keep := make([]rotatedMeta, 0, len(files))
	for _, item := range files {
		if w.maxAge > 0 && !item.ts.IsZero() && now.Sub(item.ts) > w.maxAge {
			_ = os.Remove(item.path)
			continue
		}
		keep = append(keep, item)
	}

	if w.maxFiles > 0 && len(keep) > w.maxFiles {
		for _, item := range keep[w.maxFiles:] {
			_ = os.Remove(item.path)
		}
	}

	return nil
}

func sameSliceWindow(slice string, a, b time.Time) bool {
	switch slice {
	case "year":
		return a.Year() == b.Year()
	case "month":
		return a.Year() == b.Year() && a.Month() == b.Month()
	case "day":
		ay, am, ad := a.Date()
		by, bm, bd := b.Date()
		return ay == by && am == bm && ad == bd
	case "hour":
		ay, am, ad := a.Date()
		by, bm, bd := b.Date()
		return ay == by && am == bm && ad == bd && a.Hour() == b.Hour()
	default:
		return true
	}
}
