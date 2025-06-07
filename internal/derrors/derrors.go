package derrors

import "github.com/k1LoW/errors"

func Wrap(errp *error) {
	if errp == nil || *errp == nil {
		return
	}
	*errp = errors.WithStack(*errp)
}
