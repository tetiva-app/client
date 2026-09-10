package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// deviceCodeGrant is the grant_type of the polling request (RFC 8628 §3.4).
const deviceCodeGrant = "urn:ietf:params:oauth:grant-type:device_code"

// What a device authorization response may ask of us: an interval outside this range would hammer
// the IdP or stall the flow, and an expiry past an hour outlives the user's view of the code.
const (
	minDeviceInterval = 1
	maxDeviceInterval = 300
	minDeviceExpires  = 1
	maxDeviceExpires  = 3600
	// slowDownStep is what RFC 8628 §3.5 asks a client to add per slow_down.
	slowDownStep = 5 * time.Second
)

// deviceAuth is a validated device authorization response. ExpiresAt is a local deadline, so an IdP
// that keeps answering authorization_pending past its own expiry still ends the flow.
type deviceAuth struct {
	DeviceCode              string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	Interval                time.Duration
	ExpiresAt               time.Time
}

func requestDeviceAuthorization(ctx context.Context, client *http.Client, cfg OAuth2Config,
	now time.Time, opts FlowOptions,
) (deviceAuth, error) {
	endpoint, err := ValidateEndpointURL(cfg.DeviceAuthURL)
	if err != nil {
		return deviceAuth{}, fmt.Errorf("oauth2: deviceAuthUrl: %w", err)
	}

	f, err := postForm(ctx, client, cfg, endpoint, deviceAuthForm(cfg))
	if err != nil {
		return deviceAuth{}, err
	}

	da := deviceAuth{
		DeviceCode:              f.Str("device_code"),
		UserCode:                f.Str("user_code"),
		VerificationURI:         strings.TrimSpace(f.Str("verification_uri")),
		VerificationURIComplete: strings.TrimSpace(f.Str("verification_uri_complete")),
		Interval:                opts.DefaultInterval,
		ExpiresAt:               now.Add(opts.DeviceTimeout),
	}
	if da.DeviceCode == "" || da.UserCode == "" {
		return deviceAuth{}, errors.New("oauth2: the device authorization response carried no device_code or user_code")
	}
	if err = validateVerificationURI(da.VerificationURI); err != nil {
		return deviceAuth{}, err
	}
	if da.VerificationURIComplete != "" {
		if err = validateVerificationURI(da.VerificationURIComplete); err != nil {
			return deviceAuth{}, err
		}
	}

	interval, ok, err := boundedSeconds(f, "interval", minDeviceInterval, maxDeviceInterval)
	if err != nil {
		return deviceAuth{}, err
	}
	if ok {
		da.Interval = time.Duration(interval) * time.Second
	}

	expires, ok, err := boundedSeconds(f, "expires_in", minDeviceExpires, maxDeviceExpires)
	if err != nil {
		return deviceAuth{}, err
	}
	if ok {
		da.ExpiresAt = now.Add(time.Duration(expires) * time.Second)
	}

	return da, nil
}

func deviceAuthForm(cfg OAuth2Config) url.Values {
	return withScopeAudience(cfg, url.Values{"client_id": {cfg.ClientID}})
}

func devicePollForm(cfg OAuth2Config, deviceCode string) url.Values {
	return url.Values{
		"grant_type":  {deviceCodeGrant},
		"device_code": {deviceCode},
		"client_id":   {cfg.ClientID},
	}
}

// boundedSeconds reads an optional numeric field. A present but unusable value is an error rather
// than a silent default: an IdP asking for interval "soon" cannot be polled safely.
func boundedSeconds(f Fields, key string, minSec, maxSec int64) (int64, bool, error) {
	raw, present := f[key]
	if !present || raw == nil {
		return 0, false, nil
	}

	seconds, ok := expiresInSeconds(raw)
	if !ok || seconds < minSec || seconds > maxSec {
		return 0, false, fmt.Errorf("oauth2: the device authorization response carried an unusable %s", key)
	}

	return seconds, true, nil
}

// validateVerificationURI keeps a javascript: or data: address out of the URL the
// user is invited to open.
func validateVerificationURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("oauth2: the device verification URL must be an http or https address")
	}

	return nil
}
