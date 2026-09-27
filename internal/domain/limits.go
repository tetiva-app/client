package domain

// MaxDescriptionLen is in bytes; a 100-entity sync batch must fit the 4 MiB gRPC default.
const MaxDescriptionLen = 16 * 1024

const MaxExampleBodyLen = 256 * 1024

// MaxExamplePayloadLen leaves headroom under the server's 512 KiB payload limit.
const MaxExamplePayloadLen = 480 * 1024
