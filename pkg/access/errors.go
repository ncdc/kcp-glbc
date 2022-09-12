package access

import (
	"fmt"
)

var (
	ErrInvalidAccessObject  = fmt.Errorf("not a valid access object type (expected: ingress or route)")
	ErrGeneratedHostMissing = fmt.Errorf("generated host annotation '%v' was expected but was not present", ANNOTATION_HCG_HOST)
)

func IsInvalidAccessObjectError(err error) bool {
	return err.Error() == ErrInvalidAccessObject.Error()
}

func IsMissingGeneratedHostError(err error) bool {
	return err.Error() == ErrGeneratedHostMissing.Error()
}
