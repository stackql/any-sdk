package parametertranslate

func NewNilTranslator() ParameterTranslator {
	return &NilTranslator{}
}

type NilTranslator struct {
}

func (gp *NilTranslator) Translate(input string) (string, error) {
	return input, nil
}

func (gp *NilTranslator) ReverseTranslate(input string) (string, error) {
	return input, nil
}
