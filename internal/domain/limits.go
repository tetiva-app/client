package domain

// MaxDescriptionLen bounds Markdown docs in bytes so a 100-entity sync batch fits the 4 MiB gRPC default.
const MaxDescriptionLen = 16 * 1024
