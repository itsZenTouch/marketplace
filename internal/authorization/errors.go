package authorization

import "errors"

var (
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrForbidden       = errors.New("forbidden")
)

var (
	ErrUserNotFound      = errors.New("authorization user not found")
	ErrUserSuspended     = errors.New("authorization user suspended")
	ErrUserDisabled      = errors.New("authorization user disabled")
	ErrInvalidUserStatus = errors.New("authorization invalid user status")
)
