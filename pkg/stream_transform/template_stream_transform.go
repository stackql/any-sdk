package stream_transform

import (
	"bytes"
	"encoding/json"
	"io"
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stackql/any-sdk/pkg/xmlmap"
)

var (
	_ io.ReadCloser = (*readClosableBuffer)(nil)
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

type readClosableBuffer struct {
	buf *bytes.Buffer
}

func (rcb *readClosableBuffer) Read(p []byte) (n int, err error) {
	return rcb.buf.Read(p)
}

func (rcb *readClosableBuffer) Close() error {
	return nil
}

func NewReadClosableBuffer(input string) io.ReadCloser {
	return &readClosableBuffer{
		buf: bytes.NewBufferString(input),
	}
}

type xmlReader struct {
	inStream io.ReadCloser
	schema   *openapi3.Schema
}

func NewXMLReader(inStream io.ReadCloser, schema *openapi3.Schema) ObjectReader {
	return &xmlReader{
		inStream: inStream,
		schema:   schema,
	}
}

func (xr *xmlReader) Read() (interface{}, error) {
	rv, _, err := xmlmap.GetSubObjTyped(xr.inStream, "/*", xr.schema)
	return rv, err
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
		"separator":         separator,
		"jsonMapFromString": jsonMapFromString,
		"getXPath":          getXPathInner,
		"getXPathAllOuter":  getXPathAllOuter,
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
