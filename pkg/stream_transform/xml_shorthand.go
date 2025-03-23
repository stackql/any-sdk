package stream_transform

import (
	"io"
	"strings"

	"github.com/antchfx/xmlquery"
)

var (
	_ *xmlquery.Node = (*xmlquery.Node)(nil)
)

type XMLStringShorthand interface {
	GetFirstFull(string, string) (string, error)
	GetAllFull(string, string) ([]string, error)
	GetFirstInner(string, string) (string, error)
	GetAllInner(string, string) ([]string, error)
}

func NewXMLStringShorthand() XMLStringShorthand {
	return &xmlStringShorthand{
		shorthand: NewXMLShorthand(),
	}
}

type xmlStringShorthand struct {
	shorthand XMLShorthand
}

func (xs *xmlStringShorthand) GetFirstFull(input string, path string) (string, error) {
	return xs.shorthand.GetFirstFull(strings.NewReader(input), path)
}

func (xs *xmlStringShorthand) GetAllFull(input string, path string) ([]string, error) {
	return xs.shorthand.GetAllFull(strings.NewReader(input), path)
}

func (xs *xmlStringShorthand) GetFirstInner(input string, path string) (string, error) {
	return xs.shorthand.GetFirstInner(strings.NewReader(input), path)
}

func (xs *xmlStringShorthand) GetAllInner(input string, path string) ([]string, error) {
	return xs.shorthand.GetAllInner(strings.NewReader(input), path)
}

type XMLShorthand interface {
	GetFirstFull(io.Reader, string) (string, error)
	GetAllFull(io.Reader, string) ([]string, error)
	GetFirstInner(io.Reader, string) (string, error)
	GetAllInner(io.Reader, string) ([]string, error)
}

func NewXMLShorthand() XMLShorthand {
	return &xmlShorthand{}
}

type xmlShorthand struct{}

func (xs *xmlShorthand) GetFirstFull(input io.Reader, path string) (string, error) {
	node, err := xs.getFirst(input, path)
	if err != nil {
		return "", err
	}
	return node.OutputXML(true), nil
}

func (xs *xmlShorthand) GetAllFull(input io.Reader, path string) ([]string, error) {
	nodes, err := xs.getAll(input, path)
	if err != nil {
		return nil, err
	}
	var rv []string
	for _, node := range nodes {
		rv = append(rv, node.OutputXML(true))
	}
	return rv, nil
}

func (xs *xmlShorthand) GetFirstInner(input io.Reader, path string) (string, error) {
	node, err := xs.getFirst(input, path)
	if err != nil {
		return "", err
	}
	return node.InnerText(), nil
}

func (xs *xmlShorthand) GetAllInner(input io.Reader, path string) ([]string, error) {
	nodes, err := xs.getAll(input, path)
	if err != nil {
		return nil, err
	}
	var rv []string
	for _, node := range nodes {
		rv = append(rv, node.InnerText())
	}
	return rv, nil
}

func (xs *xmlShorthand) getFirst(input io.Reader, path string) (*xmlquery.Node, error) {
	doc, err := xmlquery.Parse(input)
	if err != nil {
		return nil, err
	}
	return xmlquery.Query(doc, path)
}

func (xs *xmlShorthand) getAll(input io.Reader, path string) ([]*xmlquery.Node, error) {
	doc, err := xmlquery.Parse(input)
	if err != nil {
		return nil, err
	}
	return xmlquery.QueryAll(doc, path)
}
