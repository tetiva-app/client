package domain

// MaxDescriptionLen is in bytes; a 100-entity sync batch must fit the 4 MiB gRPC default.
const MaxDescriptionLen = 16 * 1024
