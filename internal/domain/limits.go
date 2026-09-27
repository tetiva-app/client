package domain

// MaxDescriptionLen is in bytes; a 100-entity sync batch must fit the 4 MiB gRPC default.
const MaxDescriptionLen = 16 * 1024

// MaxExampleBodyLen is in UTF-8 bytes of the body alone.
const MaxExampleBodyLen = 256 * 1024

// MaxExamplePayloadLen covers body, headers JSON, name, status text and content type; the server rejects payloads over 512 KiB.
const MaxExamplePayloadLen = 480 * 1024
