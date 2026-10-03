package appupdate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/tetiva-app/client/internal/domain/entities"
)

const progressStep = 64 << 10

var errStalled = errors.New("no data within the idle timeout")

// HTTPDownloader has no overall timeout; only IdleTimeout of silence fails a download.
type HTTPDownloader struct {
	Client      *http.Client
	IdleTimeout time.Duration
}

func NewDownloader() *HTTPDownloader {
	return &HTTPDownloader{
		Client: &http.Client{Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ResponseHeaderTimeout: 30 * time.Second,
		}},
		IdleTimeout: 30 * time.Second,
	}
}

func (d *HTTPDownloader) Download(ctx context.Context, a entities.Artifact, dst string, progress func(received int64)) error {
	const funcName = "appupdate.HTTPDownloader.Download"
	if err := d.download(ctx, a, dst, progress); err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

func (d *HTTPDownloader) download(ctx context.Context, a entities.Artifact, dst string, progress func(received int64)) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	idle := time.AfterFunc(d.IdleTimeout, func() { cancel(errStalled) })
	defer idle.Stop()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return err
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		return stalledOr(ctx, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}

	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	body := &idleReader{r: resp.Body, timer: idle, timeout: d.IdleTimeout}
	counter := &progressWriter{report: progress}
	n, err := io.Copy(io.MultiWriter(f, counter), io.LimitReader(body, a.Size+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return stalledOr(ctx, err)
	}
	if n > a.Size {
		return fmt.Errorf("body longer than the declared %d bytes", a.Size)
	}
	progress(n)
	return nil
}

func stalledOr(ctx context.Context, err error) error {
	if errors.Is(context.Cause(ctx), errStalled) {
		return errStalled
	}
	return err
}

type idleReader struct {
	r       io.Reader
	timer   *time.Timer
	timeout time.Duration
}

func (r *idleReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 {
		r.timer.Reset(r.timeout)
	}
	return n, err
}

type progressWriter struct {
	n, reported int64
	report      func(int64)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	if w.n-w.reported >= progressStep {
		w.reported = w.n
		w.report(w.n)
	}
	return len(p), nil
}
