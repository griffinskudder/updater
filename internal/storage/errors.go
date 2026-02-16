package storage

import "errors"

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrHasDependencies is returned when attempting to delete a resource that has dependent records.
var ErrHasDependencies = errors.New("resource has dependent records")
