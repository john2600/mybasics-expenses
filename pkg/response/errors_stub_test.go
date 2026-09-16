package response

import "errors"

var (
	errStub = errors.New("stub failure")
	// Shaped like a real driver error: the point is that its contents — the
	// constraint and table names — must never reach the client.
	errSecret = errors.New("error 1452: constraint fails on movements_ibfk_1")
)
