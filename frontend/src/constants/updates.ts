// Update-check manifest lives on our sync server so a release is just a file
// edit on the host; the URL is a single constant to move it later.
export const UPDATE_MANIFEST_URL = 'https://api.tetiva.app/updates/latest.json'
export const UPDATE_FALLBACK_URL = 'https://tetiva.app/download'
export const UPDATE_CHECK_INTERVAL_MS = 10 * 24 * 60 * 60 * 1000 // 10 days
