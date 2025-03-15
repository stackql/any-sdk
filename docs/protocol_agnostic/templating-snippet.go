package main

import (
	"encoding/json"
	"log"
	"os"
	"text/template"
)

func main() {
	// Define a template.
	const letter = `
{{ .parameters.executable }} req -x509 -keyout {{ .parameters.key_out_file }} -out {{ .parameters.cert_out_file }} -config {{ .parameters.config_file }} -days {{ .parameters.days }}
`

	var recipientStr = `[
		{"parameters": { "executable": "openssl", "key_out_file": "test/key.pem", "cert_out_file": "test/cert.pem", "config_file": "test/openssl.conf", "days": 365 }}
	]`

	var recipients []map[string]interface{}
	if err := json.Unmarshal([]byte(recipientStr), &recipients); err != nil {
		panic(err)
	}

	// Create a new template and parse the letter into it.
	t := template.Must(template.New("letter").Parse(letter))

	// Execute the template for each recipient.
	for _, r := range recipients {
		err := t.Execute(os.Stdout, r)
		if err != nil {
			log.Println("executing template:", err)
		}
	}

}
