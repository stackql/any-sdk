package stream_transform

import (
	"bytes"
	"encoding/json"
	"io"
	"text/template"
)

// full acknowledgment to https://stackoverflow.com/a/42663928
func separator(s string) func() string {
	i := -1
	return func() string {
		i++
		if i == 0 {
			return ""
		}
		return s
	}
}

func getXPathInner(xml string, path string) (string, error) {
	ss := NewXMLStringShorthand()
	return ss.GetFirstInner(xml, path)
}

func getRegexpFirstMatch(input string, pattern string) (string, error) {
	rs := NewRegexpShorthand()
	return rs.GetFirstMatch(input, pattern)
}

func getRegexpAllMatches(input string, pattern string) ([]string, error) {
	rs := NewRegexpShorthand()
	return rs.GetAllMatches(input, pattern)
}

func getXPathAllOuter(xml string, path string) ([]string, error) {
	ss := NewXMLStringShorthand()
	return ss.GetAllFull(xml, path)
}

type ObjectReader interface {
	Read() (interface{}, error)
}

type jsonReader struct {
	inStream io.Reader
}

func NewJSONReader(inStream io.Reader) ObjectReader {
	return &jsonReader{
		inStream: inStream,
	}
}

func (jr *jsonReader) Read() (interface{}, error) {
	var v interface{}
	err := json.NewDecoder(jr.inStream).Decode(&v)
	return v, err
}

type textReader struct {
	inStream io.Reader
}

func NewTextReader(inStream io.Reader) ObjectReader {
	return &textReader{
		inStream: inStream,
	}
}

func (tr *textReader) Read() (interface{}, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(tr.inStream)
	if err != nil {
		return "", err
	}
	rv := buf.String()
	return rv, io.EOF
}

func jsonMapFromString(s string) (map[string]interface{}, error) {
	var v map[string]interface{}
	err := json.Unmarshal([]byte(s), &v)
	return v, err
}

type StreamTransformer interface {
	Transform() error
}

type templateStreamTransfomer struct {
	tpl       *template.Template
	inStream  ObjectReader
	outStream io.Writer
}

func NewTemplateStreamTransformer(
	tplStr string,
	inStream ObjectReader,
	outStream io.Writer,
) (StreamTransformer, error) {
	return newTemplateStreamTransformer(tplStr, inStream, outStream)
}

func newTemplateStreamTransformer(
	tplStr string,
	inStream ObjectReader,
	outStream io.Writer,
) (StreamTransformer, error) {
	tpl, tplErr := template.New("__stream_tfm__").Funcs(template.FuncMap{
		"separator":           separator,
		"jsonMapFromString":   jsonMapFromString,
		"getXPath":            getXPathInner,
		"getXPathAllOuter":    getXPathAllOuter,
		"getRegexpFirstMatch": getRegexpFirstMatch,
		"getRegexpAllMatches": getRegexpAllMatches,
	}).Parse(tplStr)
	if tplErr != nil {
		return nil, tplErr
	}
	if outStream == nil {
		outStream = bytes.NewBuffer(nil)
	}
	return &templateStreamTransfomer{
		tpl:       tpl,
		inStream:  inStream,
		outStream: outStream,
	}, nil
}

func (tst *templateStreamTransfomer) Transform() error {
	for {
		obj, readErr := tst.inStream.Read()
		if obj == nil {
			if readErr != nil && readErr != io.EOF {
				return readErr
			}
			break
		}
		execErr := tst.tpl.Execute(tst.outStream, obj)
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
		if execErr == io.EOF {
			break
		}
		if execErr != nil {
			return execErr
		}
	}
	return nil
}
