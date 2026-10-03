//go:build !updatetest

package appupdate

import "crypto/ed25519"

const releaseManifestURL = "https://api.tetiva.app/updates/latest.json"

var releaseKeys = []string{"3JhaSF80CmaAzSlW7MnwtZ4Yy7ZNXHUN2hNwMeeg9+o="}

func ReleaseKeys() []ed25519.PublicKey { return decodeKeys(releaseKeys...) }

func ManifestURL(string) string { return releaseManifestURL }

func AutoApply(string) bool { return false }

func AllowLoopbackHTTP() bool { return false }
