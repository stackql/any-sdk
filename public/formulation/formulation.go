package formulation

import (
	"github.com/stackql/any-sdk/anysdk"
)

type ArmouryGenerator interface {
	GetHTTPArmoury() (anysdk.HTTPArmoury, error)
}
