package errorz

import "errors"

var (
	ErrInternalServerError = errors.New("internal server error")
	ErrCaptchaNotFound     = errors.New("captcha not found")
	ErrCaptchaForbidden    = errors.New("captcha forbidden")
	ErrCaptchaGone         = errors.New("captcha gone")
)
