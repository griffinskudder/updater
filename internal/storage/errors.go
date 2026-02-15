package storage

import "errors"

// ErrHasDependencies is returned when attempting to delete a resource that has dependent records.
var ErrHasDependencies = errors.New("resource has dependent records")
