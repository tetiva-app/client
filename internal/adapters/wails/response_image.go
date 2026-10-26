package wails

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
)

// Sent as base64 JSON and decoded again in JS, so a preview costs several times its size.
const maxPreviewImageBytes = 10 << 20

const (
	ReasonPreviewTooLarge = "PREVIEW_TOO_LARGE"
	ReasonPreviewNotImage = "PREVIEW_NOT_IMAGE"
)

const responseFilePrefix = "gopher-response-"

var rasterImageTypes = map[string]bool{
	"image/png":    true,
	"image/jpeg":   true,
	"image/gif":    true,
	"image/webp":   true,
	"image/bmp":    true,
	"image/x-icon": true,
}

func (s *RequestService) ReadResponseImage(tempPath string, contentType string) Result[dto.ResponseImageDTO] {
	const funcName = "RequestService.ReadResponseImage"

	path, tempDir, err := resolveTempPath(tempPath)
	if err != nil {
		return Err[dto.ResponseImageDTO](fmt.Errorf("%s: %w", funcName, err))
	}
	if filepath.Dir(path) != tempDir || !strings.HasPrefix(filepath.Base(path), responseFilePrefix) {
		return Err[dto.ResponseImageDTO](fmt.Errorf("%s: not a response file", funcName))
	}

	f, err := os.Open(path)
	if err != nil {
		return Err[dto.ResponseImageDTO](fmt.Errorf("%s: %w", funcName, err))
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, maxPreviewImageBytes+1))
	if err != nil {
		return Err[dto.ResponseImageDTO](fmt.Errorf("%s: %w", funcName, err))
	}

	mimeType := previewImageType(contentType, data)
	if mimeType == "" {
		return Err[dto.ResponseImageDTO](&domain.ReasonError{
			Reason: ReasonPreviewNotImage,
			Err:    fmt.Errorf("%s: not a previewable image", funcName),
		})
	}
	if len(data) > maxPreviewImageBytes {
		return Err[dto.ResponseImageDTO](&domain.ReasonError{
			Reason: ReasonPreviewTooLarge,
			Err:    fmt.Errorf("%s: image is too large to preview", funcName),
			Fields: map[string]string{"limit": strconv.Itoa(maxPreviewImageBytes)},
		})
	}

	return OK(dto.ResponseImageDTO{MimeType: mimeType, Data: data})
}

func previewImageType(contentType string, data []byte) string {
	if sniffed, _, _ := mime.ParseMediaType(http.DetectContentType(data)); rasterImageTypes[sniffed] {
		return sniffed
	}
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil && mediaType == "image/svg+xml" && looksLikeSVG(data) {
		return mediaType
	}
	return ""
}

func looksLikeSVG(data []byte) bool {
	head := bytes.TrimPrefix(data[:min(len(data), 4096)], []byte("\xEF\xBB\xBF"))
	for {
		head = bytes.TrimLeft(head, " \t\r\n")
		var end []byte
		switch {
		case bytes.HasPrefix(head, []byte("<?")):
			end = []byte("?>")
		case bytes.HasPrefix(head, []byte("<!--")):
			end = []byte("-->")
		case bytes.HasPrefix(head, []byte("<!DOCTYPE svg")):
			end = []byte(">")
		default:
			rest, ok := bytes.CutPrefix(head, []byte("<svg"))
			return ok && len(rest) > 0 && bytes.IndexByte([]byte(" \t\r\n/>"), rest[0]) >= 0
		}
		i := bytes.Index(head, end)
		if i < 0 {
			return false
		}
		head = head[i+len(end):]
	}
}
