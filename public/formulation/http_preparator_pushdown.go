package formulation

import (
	"net/url"

	"github.com/stackql/any-sdk/internal/anysdk"
)

// queryParamSettable is the minimal surface needed to merge query params onto a
// built request. anysdk.HTTPArmouryParameters satisfies it.
type queryParamSettable interface {
	GetQuery() url.Values
	SetRawQuery(string)
}

// applyQueryParams merges qp onto each param's query string. It is a no-op when qp
// is empty, leaving the request byte-for-byte unchanged.
func applyQueryParams(params []queryParamSettable, qp map[string]string) {
	if len(qp) == 0 {
		return
	}
	for _, p := range params {
		q := p.GetQuery()
		for k, v := range qp {
			q.Set(k, v)
		}
		p.SetRawQuery(q.Encode())
	}
}

// applyPushdownToArmoury translates the neutral intent against the source's
// queryParamPushdown config and sets the resulting query params on every request in
// the armoury. Absent config (or no translatable options) yields zero params and
// leaves the armoury untouched, preserving existing behaviour.
func applyPushdownToArmoury(armoury anysdk.HTTPArmoury, src PushdownConfigSource, intent PushdownIntent) {
	res := ApplyPushdown(src, intent)
	qp := res.QueryParams()
	if len(qp) == 0 {
		return
	}
	inner := armoury.GetRequestParams()
	settable := make([]queryParamSettable, 0, len(inner))
	for _, p := range inner {
		settable = append(settable, p)
	}
	applyQueryParams(settable, qp)
}
