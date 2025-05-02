package stream_transform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"text/template"

	"github.com/clbanning/mxj/v2"
)

const (
	GolangTemplateXMLV1  = "golang_template_mxj_v0.1.0"
	GolangTemplateJSONV1 = "golang_template_json_v0.1.0"
	GolangTemplateTextV1 = "golang_template_text_v0.1.0"
)

type StreamTransformerFactory interface {
	IsTransformable() bool
	GetTransformer(input string) (StreamTransformer, error)
}

type streamTransformerFactory struct {
	tplType string
	tplStr  string
}

func NewStreamTransformerFactory(tplType string, tplStr string) StreamTransformerFactory {
	return &streamTransformerFactory{
		tplType: tplType,
		tplStr:  tplStr,
	}
}

func (stf *streamTransformerFactory) IsTransformable() bool {
	switch stf.tplType {
	case GolangTemplateXMLV1:
		return true
	case GolangTemplateJSONV1:
		return true
	case GolangTemplateTextV1:
		return true
	default:
		return false
	}
}

func (stf *streamTransformerFactory) GetTransformer(input string) (StreamTransformer, error) {
	switch stf.tplType {
	case GolangTemplateXMLV1:
		inStream := newXMLBestEffortReader(bytes.NewBufferString(input))
		outStream := bytes.NewBuffer(nil)
		tfm, err := newTemplateStreamTransformer(stf.tplStr, inStream, outStream)
		return tfm, err
	case GolangTemplateJSONV1:
		inStream := newJSONReader(bytes.NewBufferString(input))
		outStream := bytes.NewBuffer(nil)
		tfm, err := newTemplateStreamTransformer(stf.tplStr, inStream, outStream)
		return tfm, err
	case GolangTemplateTextV1:
		inStream := newTextReader(bytes.NewBufferString(input))
		outStream := bytes.NewBuffer(nil)
		tfm, err := newTemplateStreamTransformer(stf.tplStr, inStream, outStream)
		return tfm, err
	default:
		return nil, fmt.Errorf("unsupported template type: %s", stf.tplType)
	}
}

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

func toBool(v interface{}) bool {
	switch v := v.(type) {
	case string:
		return v == "true"
	case bool:
		return v
	default:
		return false
	}
}

func toInt(v interface{}) int {
	switch v := v.(type) {
	case string:
		var i int
		fmt.Sscanf(v, "%d", &i)
		return i
	case float64:
		return int(v)
	default:
		return 0
	}
}

func safeIndex(m map[string]interface{}, key string) interface{} {
	if m == nil {
		return nil
	}
	return m[key]
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

func newJSONReader(inStream io.Reader) ObjectReader {
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

type xmlBestEffortReader struct {
	inStream io.Reader
}

func (xer *xmlBestEffortReader) Read() (interface{}, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(xer.inStream)
	if err != nil {
		return nil, err
	}
	inBytes := buf.Bytes()
	mv, err := mxj.NewMapXml(inBytes, true) // true = strip namespaces
	if err != nil {
		return nil, err
	}
	return mv, nil
}

func newXMLBestEffortReader(inStream io.Reader) ObjectReader {
	return &xmlBestEffortReader{
		inStream: inStream,
	}
}

func newTextReader(inStream io.Reader) ObjectReader {
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
	GetOutStream() io.Reader
}

type templateStreamTransfomer struct {
	tpl       *template.Template
	inStream  ObjectReader
	outStream io.ReadWriter
}

// func NewTemplateStreamTransformer(
// 	tplStr string,
// 	inStream ObjectReader,
// 	outStream io.Writer,
// ) (StreamTransformer, error) {
// 	return newTemplateStreamTransformer(tplStr, inStream, outStream)
// }

func newTemplateStreamTransformer(
	tplStr string,
	inStream ObjectReader,
	outStream io.ReadWriter,
) (StreamTransformer, error) {
	tpl, tplErr := template.New("__stream_tfm__").Funcs(template.FuncMap{
		"separator":           separator,
		"jsonMapFromString":   jsonMapFromString,
		"getXPath":            getXPathInner,
		"getXPathAllOuter":    getXPathAllOuter,
		"getRegexpFirstMatch": getRegexpFirstMatch,
		"getRegexpAllMatches": getRegexpAllMatches,
		"safeIndex":           safeIndex,
		"toBool":              toBool,
		"toInt":               toInt,
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

func (tst *templateStreamTransfomer) GetOutStream() io.Reader {
	if tst.outStream == nil {
		return bytes.NewBuffer(nil)
	}
	return tst.outStream
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
