// Package data holds domain models shared across packages that would otherwise
// import each other. Keeping them here (a leaf package that imports nothing
// internal) breaks import cycles — e.g. security can return a User without
// importing the users package.
package data

// User is the shared user view returned by cross-package lookups (e.g. the
// token → user join in security). It is intentionally light; the users package
// keeps its own richer model for user CRUD.
type User struct {
	ID        int
	Username  string
	Name      string
	Email     string
	Activated bool
}
