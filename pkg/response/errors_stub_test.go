package response

import "errors"

var (
	errStub   = errors.New("stub failure")
	errSecret = errors.New("Error 1452: constraint fails on movements_ibfk_1")
)
