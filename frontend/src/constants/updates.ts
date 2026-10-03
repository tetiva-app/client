// Update-check manifest lives on our sync server so a release is just a file
// edit on the host; the URL is a single constant to move it later.
export const UPDATE_MANIFEST_URL = 'https://api.tetiva.app/updates/latest.json'
export const UPDATE_FALLBACK_URL = 'https://tetiva.app/download'
export const UPDATE_CHECK_INTERVAL_MS = 24 * 60 * 60 * 1000

export const APT_UPGRADE_COMMAND = 'sudo apt update && sudo apt install --only-upgrade tetiva'
export const APT_SETUP_COMMANDS = [
  'wget -qO- https://apt.tetiva.app/tetiva.gpg | sudo tee /etc/apt/keyrings/tetiva.gpg >/dev/null',
  'echo "deb [signed-by=/etc/apt/keyrings/tetiva.gpg] https://apt.tetiva.app stable main" | sudo tee /etc/apt/sources.list.d/tetiva.list',
  'sudo apt update && sudo apt install tetiva',
].join('\n')
