package marshalmap

import (
	"encoding/xml"
	"fmt"
)

// AnyMap is a map[string]any.
type AnyMap map[string]any

// StringMap marshals into XML.
func (s AnyMap) MarshalXML(e *xml.Encoder, start xml.StartElement) error {

	tokens := []xml.Token{start}

	for key, value := range s {
		sVal := fmt.Sprintf("%v", value)
		t := xml.StartElement{Name: xml.Name{"", key}}
		tokens = append(tokens, t, xml.CharData(sVal), xml.EndElement{t.Name})
	}

	tokens = append(tokens, xml.EndElement{start.Name})

	for _, t := range tokens {
		err := e.EncodeToken(t)
		if err != nil {
			return err
		}
	}

	// flush to ensure tokens are written
	return e.Flush()
}
