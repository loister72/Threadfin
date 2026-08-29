package src

import (
	"fmt"
	"io"
	"os"
	"time"
)

const (
	maxThirdPartySegmentSize        = 128 * 1024
	maxThirdPartyStartupSegmentSize = 32 * 1024
	defaultThirdPartyStartupTimeout = 20 * time.Second
	maxThirdPartyStartupTimeout     = 120 * time.Second
)

func thirdPartyStartupTimeout() time.Duration {
	if Settings.BufferTimeout <= 0 {
		return defaultThirdPartyStartupTimeout
	}

	timeout := time.Duration(Settings.BufferTimeout * float64(time.Second))
	if timeout < time.Second {
		return time.Second
	}
	if timeout > maxThirdPartyStartupTimeout {
		return maxThirdPartyStartupTimeout
	}

	return timeout
}

type ThirdPartySegmentWriter struct {
	folder       string
	segment      int
	fileSize     int
	rotateAtSize int
	file         io.WriteCloser
}

func NewThirdPartySegmentWriter(folder string, bufferSizeBytes int) *ThirdPartySegmentWriter {
	return &ThirdPartySegmentWriter{
		folder:       folder,
		segment:      1,
		rotateAtSize: thirdPartySegmentRotateSize(bufferSizeBytes),
	}
}

func thirdPartySegmentRotateSize(bufferSizeBytes int) int {
	rotateAtSize := bufferSizeBytes / 2
	if rotateAtSize < 1 {
		return 1
	}
	if rotateAtSize > maxThirdPartySegmentSize {
		return maxThirdPartySegmentSize
	}
	return rotateAtSize
}

func (w *ThirdPartySegmentWriter) Reset() error {
	if err := bufferVFS.RemoveAll(getPlatformPath(w.folder)); err != nil {
		ShowError(err, 4005)
	}

	return checkVFSFolder(w.folder, bufferVFS)
}

func (w *ThirdPartySegmentWriter) CreateCurrent() error {
	f, err := bufferVFS.Create(w.currentPath())
	if err != nil {
		return err
	}

	return f.Close()
}

func (w *ThirdPartySegmentWriter) OpenCurrent() error {
	f, err := bufferVFS.OpenFile(w.currentPath(), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	w.file = f
	return nil
}

func (w *ThirdPartySegmentWriter) Write(p []byte) (int, error) {
	if w.file == nil {
		return 0, fmt.Errorf("segment writer is not open")
	}

	n, err := w.file.Write(p)
	w.fileSize += n
	return n, err
}

func (w *ThirdPartySegmentWriter) ShouldRotate() bool {
	return w.fileSize >= w.currentRotateSize()
}

func (w *ThirdPartySegmentWriter) Rotate() error {
	if err := w.Close(); err != nil {
		return err
	}

	w.segment++
	w.fileSize = 0

	if err := w.CreateCurrent(); err != nil {
		return err
	}

	return w.OpenCurrent()
}

func (w *ThirdPartySegmentWriter) Close() error {
	if w.file == nil {
		return nil
	}

	err := w.file.Close()
	w.file = nil
	return err
}

func (w *ThirdPartySegmentWriter) CurrentSize() int {
	return w.fileSize
}

func (w *ThirdPartySegmentWriter) Segment() int {
	return w.segment
}

func (w *ThirdPartySegmentWriter) currentRotateSize() int {
	if w.segment == 1 && w.rotateAtSize > maxThirdPartyStartupSegmentSize {
		return maxThirdPartyStartupSegmentSize
	}
	return w.rotateAtSize
}

func (w *ThirdPartySegmentWriter) currentPath() string {
	return fmt.Sprintf("%s%d.ts", w.folder, w.segment)
}
