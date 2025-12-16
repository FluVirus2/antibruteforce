package service

import "errors"

var (
	ErrSubnetCheckFailed = errors.New("failed to check IP in subnets")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	ErrSubnetNotFound  = errors.New("subnet not found")
	ErrBucketNotFound  = errors.New("bucket not found")
	ErrInvalidCIDR     = errors.New("invalid CIDR format")
	ErrInvalidIP       = errors.New("invalid IP address")
	ErrInvalidLogin    = errors.New("invalid login")
	ErrInvalidPassword = errors.New("invalid password")
)
