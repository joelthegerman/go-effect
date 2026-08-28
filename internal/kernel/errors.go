package kernel

import "errors"

// ErrNotFound is returned by a feature repository when a requested row does not
// exist. It lives in the framework (not a feature) so the shared HTTP error
// mapper can translate it to 404 without importing any feature package.
var ErrNotFound = errors.New("not found")
