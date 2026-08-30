package model

// Revision is a content hash: "sha256:" plus lowercase hex of SHA-256 of
// canonical JSON. Hashing is performed by config, not by this package.
type Revision string

// RevisionPrefix is the required prefix of every Revision value.
const RevisionPrefix = "sha256:"

// Generation is a process-local, monotonically increasing snapshot counter.
type Generation uint64
