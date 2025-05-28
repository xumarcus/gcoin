package util

func Last[E any](s []E) *E {
	n := len(s)
	if n == 0 {
		return nil
	}
	return &s[n-1]
}

type Validated interface {
	Validate() error
}
