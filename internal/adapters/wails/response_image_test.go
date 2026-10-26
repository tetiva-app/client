package wails

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 3, 2))))
	return buf.Bytes()
}

func writeResponseFile(t *testing.T, pattern string, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp("", pattern)
	require.NoError(t, err)
	_, err = f.Write(data)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	return f.Name()
}

func TestRequestService_ReadResponseImage_SniffsWithoutContentType(t *testing.T) {
	path := writeResponseFile(t, "gopher-response-*", tinyPNG(t))

	for _, ct := range []string{"", "application/octet-stream", "image/jpg"} {
		res := (&RequestService{}).ReadResponseImage(path, ct)
		require.Nil(t, res.Error, ct)
		assert.Equal(t, "image/png", res.Data.MimeType, ct)
	}
}

func TestRequestService_ReadResponseImage_SVGByContentType(t *testing.T) {
	for _, svg := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg" width="4" height="4"/>`,
		"\xEF\xBB\xBF<?xml version=\"1.0\"?>\n<!-- logo -->\n<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.1//EN\" \"x.dtd\">\n<svg>\n</svg>",
	} {
		path := writeResponseFile(t, "gopher-response-*", []byte(svg))

		res := (&RequestService{}).ReadResponseImage(path, "Image/SVG+XML; charset=utf-8")

		require.Nil(t, res.Error, svg)
		assert.Equal(t, "image/svg+xml", res.Data.MimeType, svg)
		assert.Equal(t, []byte(svg), res.Data.Data, svg)
	}
}

func TestRequestService_ReadResponseImage_RejectsOtherTypes(t *testing.T) {
	cases := map[string]struct {
		data []byte
		ct   string
	}{
		"zip without type":       {[]byte("PK\x03\x04\x14\x00\x00\x00"), ""},
		"html said to be binary": {[]byte("<!doctype html><script>alert(1)</script>"), "application/octet-stream"},
		"tiff":                   {[]byte("II*\x00\x08\x00\x00\x00"), "image/tiff"},
		"text said to be png":    {[]byte("SECRET=hunter2\n"), "image/png"},
		"html said to be svg":    {[]byte("<!doctype html><svg></svg>"), "image/svg+xml"},
		"svg tag too deep":       {append(bytes.Repeat([]byte(" "), 4096), "<svg/>"...), "image/svg+xml"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeResponseFile(t, "gopher-response-*", tc.data)

			res := (&RequestService{}).ReadResponseImage(path, tc.ct)

			require.NotNil(t, res.Error)
			assert.Equal(t, ReasonPreviewNotImage, res.Error.Reason)
			assert.Nil(t, res.Data.Data)
		})
	}
}

func TestRequestService_ReadResponseImage_TooLarge(t *testing.T) {
	data := append(tinyPNG(t), make([]byte, maxPreviewImageBytes)...)
	path := writeResponseFile(t, "gopher-response-*", data)

	res := (&RequestService{}).ReadResponseImage(path, "image/png")

	require.NotNil(t, res.Error)
	assert.Equal(t, ReasonPreviewTooLarge, res.Error.Reason)
	assert.Equal(t, strconv.Itoa(maxPreviewImageBytes), res.Error.Fields["limit"])
	assert.Nil(t, res.Data.Data)
}

func TestRequestService_ReadResponseImage_AtLimit(t *testing.T) {
	head := tinyPNG(t)
	data := append(head, make([]byte, maxPreviewImageBytes-len(head))...)
	path := writeResponseFile(t, "gopher-response-*", data)

	res := (&RequestService{}).ReadResponseImage(path, "image/png")

	require.Nil(t, res.Error)
	assert.Len(t, res.Data.Data, maxPreviewImageBytes)
}

func TestRequestService_ReadResponseImage_MissingFile(t *testing.T) {
	dir, err := filepath.EvalSymlinks(os.TempDir())
	require.NoError(t, err)

	res := (&RequestService{}).ReadResponseImage(filepath.Join(dir, "gopher-response-gone"), "image/png")

	require.NotNil(t, res.Error)
	assert.Empty(t, res.Error.Reason)
}

func TestRequestService_ReadResponseImage_OnlyResponseFiles(t *testing.T) {
	other := writeResponseFile(t, "gopher-other-*", tinyPNG(t))

	for _, path := range []string{"/etc/hosts", other, ""} {
		res := (&RequestService{}).ReadResponseImage(path, "image/png")
		require.NotNil(t, res.Error, path)
		assert.Nil(t, res.Data.Data, path)
	}
}

func TestRequestService_ReadResponseImage_OnlyDirectlyInTempDir(t *testing.T) {
	dir, err := os.MkdirTemp("", "other-app-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	nested := filepath.Join(dir, "gopher-response-x")
	require.NoError(t, os.WriteFile(nested, tinyPNG(t), 0o600))

	res := (&RequestService{}).ReadResponseImage(nested, "image/png")

	require.NotNil(t, res.Error)
	assert.Nil(t, res.Data.Data)
}
