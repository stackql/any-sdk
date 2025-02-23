//nolint:revive,stylecheck // permissable deviation from norm
package anysdk

var (
	_ HttpPreparatorStream = &httpPreparatorStream{}
)

type HttpPreparatorStream interface {
	Write(HTTPPreparator) error
	Next() (HTTPPreparator, bool)
}

type httpPreparatorStream struct {
	sl []HTTPPreparator
}

func NewHttpPreparatorStream() HttpPreparatorStream {
	return &httpPreparatorStream{}
}

func (s *httpPreparatorStream) Write(p HTTPPreparator) error {
	s.sl = append(s.sl, p)
	return nil
}

func (s *httpPreparatorStream) Next() (HTTPPreparator, bool) {
	if len(s.sl) < 1 {
		return nil, false
	}
	p := s.sl[0]
	s.sl = s.sl[1:]
	return p, true
}
