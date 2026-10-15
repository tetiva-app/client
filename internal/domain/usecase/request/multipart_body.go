package request

import (
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
)

// multipartBody streams a form with files from disk; every open reads them again.
type multipartBody struct {
	boundary string
	fields   []formField
	size     int64
}

func newMultipartBody(fields []formField) (*multipartBody, error) {
	b := &multipartBody{boundary: multipart.NewWriter(nil).Boundary()}
	var fileBytes int64
	for _, f := range fields {
		if !f.Enabled || f.Key == "" {
			continue
		}
		if f.Type == "file" && f.Value != "" {
			if strings.Contains(f.Value, "..") {
				return nil, fmt.Errorf("form file path traversal is not allowed: %s", f.Key)
			}
			f.Value = filepath.Clean(f.Value)
			if !filepath.IsAbs(f.Value) {
				return nil, fmt.Errorf("form file path must be absolute: %s", f.Key)
			}
			info, err := statBodyFile(f.Value)
			if err != nil {
				return nil, fmt.Errorf("form file %q: %w", f.Key, err)
			}
			fileBytes += info.Size()
		}
		b.fields = append(b.fields, f)
	}

	// multipart.Writer writes straight through, so skipping the file bytes leaves only the framing.
	var framing countingWriter
	if err := b.write(&framing, func(io.Writer, string) error { return nil }); err != nil {
		return nil, err
	}
	b.size = framing.n + fileBytes
	return b, nil
}

func (b *multipartBody) contentType() string {
	return "multipart/form-data; boundary=" + b.boundary
}

// The caller closes the stream; an unread one parks a goroutine until then.
func (b *multipartBody) open() io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(b.write(pw, copyBodyFile))
	}()
	return pr
}

func (b *multipartBody) write(w io.Writer, copyFile func(dst io.Writer, path string) error) error {
	mw := multipart.NewWriter(w)
	if err := mw.SetBoundary(b.boundary); err != nil {
		return fmt.Errorf("failed to set multipart boundary: %w", err)
	}
	for _, f := range b.fields {
		if f.Type != "file" || f.Value == "" {
			if err := mw.WriteField(f.Key, f.Value); err != nil {
				return fmt.Errorf("failed to write field %q: %w", f.Key, err)
			}
			continue
		}
		part, err := mw.CreateFormFile(f.Key, filepath.Base(f.Value))
		if err != nil {
			return fmt.Errorf("failed to create form file part %q: %w", f.Key, err)
		}
		if err := copyFile(part, f.Value); err != nil {
			return fmt.Errorf("form file %q: %w", f.Key, err)
		}
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}
	return nil
}

func copyBodyFile(dst io.Writer, path string) error {
	f, err := openBodyFile(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(dst, f)
	return err
}

type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}
