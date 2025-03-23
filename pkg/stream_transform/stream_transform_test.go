package stream_transform_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	. "github.com/stackql/any-sdk/pkg/stream_transform"
)

var (
	_           io.Reader = &bytes.Buffer{}
	_           io.Writer = &bytes.Buffer{}
	jsonExample           = `{ 
	  "animals": [
		{"name": "Platypus", "order": "Monotremata", "votes": 1, "bank_balance": 100.0},
		{"name": "Quokka", "order": "Diprotodontia", "votes": 3, "bank_balance": 200.0},
		{"name": "Quoll", "order": "Dasyuromorphia", "votes": 2, "bank_balance": 300.0, "premierships": [1993, 2000]}
	  ],
	  "meta": {
	    "institution": "University of Tasmania",
		"total_votes": 6,
		"total_bank_balance": 600.0
	  }
	}`
	xmlExample = `<?xml version="1.0" encoding="UTF-8"?>
	<root>
	  <animals>
	 		<animal>
				<name>Platypus</name>
				<order>Monotremata</order>
				<votes>1</votes>
				<bank_balance>100.0</bank_balance>
			</animal>
			<animal>
				<name>Quokka</name>
				<order>Diprotodontia</order>
				<votes>3</votes>
				<bank_balance>200.0</bank_balance>
			</animal>
			<animal>
				<name>Quoll</name>
				<order>Dasyuromorphia</order>
				<votes>2</votes>
				<bank_balance>300.0</bank_balance>
				<premierships>
					<premiership>1993</premiership>
					<premiership>2000</premiership>
				</premierships>
			</animal>
		</animals>
		<meta>
			<institution>University of Tasmania</institution>
			<total_votes>6</total_votes>
			<total_bank_balance>600.0</total_bank_balance>
		</meta>
	</root>  	
	  `
	yamlExample = `---
animals:
	- name: Platypus
	  order: Monotremata
	  votes: 1
	  bank_balance: 100.0				
	- name: Quokka
	  order: Diprotodontia
	  votes: 3
	  bank_balance: 200.0
	- name: Quoll
	  order: Dasyuromorphia
	  votes: 2
	  bank_balance: 300.0
	  premierships:
		- 1993
		- 2000
meta:	
	institution: University of Tasmania
	total_votes: 6
	total_bank_balance: 600.0
`
	jsonTmpl = `
	{{- $s := separator ", " -}}
	[
	{{- range $idx, $animal := $.animals -}}
	{{- call $s}}{"name": "{{ $animal.name }}", "democratic_votes": {{ $animal.votes }}} 
	{{- end -}}
	]`
	xmlTmpl               = `[{ "name": "{{- getXPath . "/root/animals/animal/name" }}"}]`
	expectedJsonOutput    = `[{"name": "Platypus", "democratic_votes": 1}, {"name": "Quokka", "democratic_votes": 3}, {"name": "Quoll", "democratic_votes": 2}]`
	openSSLCertTextOutput = `Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number:
            53:f4:3b:da:df:42:7b:bf:c3:14:08:9a:69:6d:a1:7b:47:2d:8b:8a
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: C=AU, ST=VIC, L=Melbourne, O=StackQL, OU=Core Functions, CN=127.0.0.1, emailAddress=krimmer@stackql.io
        Validity
            Not Before: Mar 22 02:50:46 2025 GMT
            Not After : Jun 20 02:50:46 2025 GMT
        Subject: C=AU, ST=VIC, L=Melbourne, O=StackQL, OU=Core Functions, CN=127.0.0.1, emailAddress=krimmer@stackql.io
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
                Public-Key: (4096 bit)
                Modulus:
                    00:b8:55:1a:d3:ac:e8:5c:19:a9:24:be:ee:36:47:
                    79:92:c9:56:e1:40:92:40:51:13:03:7b:7b:b4:00:
                    ac:70:e3:80:bf:8a:0a:86:d8:27:0d:97:7f:d9:4c:
                    4d:fd:b0:4c:9c:d7:22:a7:5b:1e:c2:4c:be:bf:e5:
                    74:b7:e7:00:a9:ad:0e:a5:02:fd:c8:62:df:8b:c2:
                    87:de:eb:54:7f:c4:69:ae:e1:f2:e5:e6:2b:9f:34:
                    0c:24:7e:3c:b4:b1:75:11:1c:16:c7:5e:0d:32:b8:
                    7e:bd:3c:0c:7d:0d:5f:84:55:9d:be:16:e3:bd:04:
                    c4:ba:9c:cd:8e:a9:56:a9:67:7c:79:60:22:c7:4c:
                    46:de:52:97:2d:fc:7e:67:3b:c5:ae:3e:9c:c3:c0:
                    be:b6:a1:82:be:5b:f5:1b:f2:a9:87:ea:ad:0d:bf:
                    b9:21:dc:dd:cf:70:d3:89:d0:8f:ab:f5:9c:67:3f:
                    d8:e3:93:80:55:3c:46:08:1a:90:20:40:2f:84:e9:
                    7d:b7:b4:4f:0b:80:80:b5:cc:51:92:6d:d0:12:f5:
                    e1:aa:2a:a7:3f:1c:2b:23:6b:92:b1:ad:cd:35:e2:
                    98:6e:f1:e9:60:85:60:aa:53:41:f7:91:b2:ac:b9:
                    83:8f:ca:44:0a:d0:53:4f:dc:15:89:54:1d:43:85:
                    67:cf:f8:da:39:09:02:8c:0d:3a:e8:f0:4e:a2:1d:
                    5a:54:d6:5f:87:9a:3a:11:4e:ad:85:4a:b6:f7:c2:
                    e3:e9:e0:d0:10:fa:3d:c2:59:98:80:0c:b7:40:71:
                    05:df:f5:72:a6:54:a2:5b:82:39:58:dd:17:72:44:
                    b1:15:03:f2:7a:26:0a:e0:db:83:1a:51:d1:1c:37:
                    c5:8d:dc:1e:72:b5:1a:d7:24:fc:4c:c6:17:84:54:
                    a4:65:3a:44:ec:11:dd:fc:ca:fd:20:fc:f7:25:01:
                    5c:38:af:66:bf:d8:c2:47:53:a6:e9:cb:52:32:8f:
                    d5:10:45:7c:0c:c1:54:3c:3a:e6:eb:50:22:b6:f2:
                    66:94:a0:1b:4e:c1:3d:32:3d:d4:a3:09:97:ed:aa:
                    d9:13:e4:5f:64:b9:d0:5f:ca:6b:b7:6c:98:8c:80:
                    86:26:6f:24:d5:19:de:11:29:e1:91:a0:45:03:7b:
                    fc:38:e2:a8:b3:c5:34:e2:e3:00:79:33:6d:57:1a:
                    1e:e7:a6:a9:3d:07:c5:6c:b7:67:c6:f5:db:d0:4d:
                    5d:8c:7c:06:b7:33:80:14:5a:b4:cf:43:4c:05:cf:
                    61:80:85:7a:46:4c:e7:7c:0a:00:dd:ce:d6:cf:13:
                    1e:28:a1:6b:66:17:fb:7f:77:83:10:20:49:f9:3b:
                    71:70:31
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            X509v3 Subject Key Identifier: 
                DB:F5:C4:59:02:8E:D5:3E:58:16:E3:C6:AD:78:A9:00:68:16:8E:DD
    Signature Algorithm: sha256WithRSAEncryption
    Signature Value:
        a2:4e:1b:97:c1:ce:7a:16:8b:1b:bb:4a:17:f9:9f:bd:95:31:
        af:84:05:51:cf:52:d1:17:96:02:87:f0:26:0b:0d:40:85:fb:
        44:d6:0c:76:3d:60:fb:f7:c0:f6:3f:7a:64:1d:6e:82:01:2d:
        6e:aa:46:dd:3b:af:34:e4:cb:ca:50:78:08:2f:98:e8:ed:c0:
        1e:65:71:14:c2:1f:e0:cb:d7:e9:43:5b:b6:60:c5:de:d3:65:
        2e:b1:51:31:25:28:73:fd:a8:96:e1:b0:a9:ef:b3:4d:dd:2c:
        89:9a:80:38:59:54:55:52:a6:8e:9f:1b:50:c1:e1:8b:44:66:
        dc:43:b8:eb:ac:d6:aa:e9:17:7b:b0:61:1b:41:65:83:23:9c:
        0a:b2:9f:1b:c4:e9:06:a3:ad:43:f2:e3:4a:3c:29:6d:c6:72:
        03:59:79:87:f4:de:86:46:2c:cc:80:c1:bd:bd:7d:f6:41:fa:
        5f:e3:6b:c4:34:ed:10:ae:59:1f:bb:c4:c3:70:22:c9:ee:ef:
        16:e2:fb:17:40:ca:71:9c:91:76:8e:00:bd:4b:d3:6c:63:f1:
        30:9f:3c:6c:9c:ad:e7:c4:37:34:9e:ec:a2:50:d3:c5:44:5b:
        af:27:c3:cd:70:ee:b3:ea:1c:aa:6f:cd:69:85:f5:9c:4b:e9:
        b5:68:12:c8:59:78:84:1f:5f:51:59:e3:38:1a:17:8b:76:54:
        f2:dd:dc:e9:d5:5a:a0:45:5a:21:e3:0b:03:05:90:ab:a7:57:
        d9:e5:62:2a:0d:ed:85:60:00:d6:b2:e0:75:4b:90:4d:a7:03:
        66:1c:29:12:14:8c:3f:06:15:d6:01:62:e2:49:d3:92:e6:cf:
        13:dd:5e:3a:8b:2f:10:8d:ca:27:d9:33:bc:0d:11:17:5f:05:
        8e:ae:f5:a0:12:36:07:8f:f3:e3:22:f9:d0:43:68:40:17:2c:
        9b:7e:bc:b9:5e:8e:b2:49:45:78:e8:ba:b9:85:4d:dd:e7:5e:
        27:e9:da:14:be:29:a4:2b:02:53:83:a1:11:08:43:f5:4e:9d:
        16:84:f2:64:a8:e3:49:d7:6a:dd:33:32:49:a8:b6:cf:8e:14:
        d1:2b:e2:61:8c:ec:f1:3a:d7:8d:ee:74:0e:26:71:e9:4e:4f:
        4c:66:4a:5a:ee:8f:56:69:1a:22:11:cd:e1:b2:fa:69:3d:d9:
        07:51:74:af:bb:12:22:d8:4b:43:aa:41:3b:de:6d:13:45:5c:
        c3:c6:9a:54:5a:c4:90:40:e1:c7:df:9d:3b:da:15:2e:0d:d2:
        73:21:b4:62:a7:4a:3f:8a:66:8e:02:38:d5:50:5c:d4:96:86:
        a5:66:21:c3:39:6f:54:cc
`
)

func TestSimpleStreamTransform(t *testing.T) {
	input := fmt.Sprintf(`"Hello, %s!"`, "World")
	t.Log("TestSimpleStream")
	tmpl := `{{.}}`
	inStream := NewJSONReader(bytes.NewBufferString(input))
	outStream := bytes.NewBuffer(nil)
	tfm, err := NewTemplateStreamTransformer(tmpl, inStream, outStream)
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}
	if err := tfm.Transform(); err != nil {
		t.Fatalf("failed to transform: %v", err)
	}
	outputStr := outStream.String()
	if outputStr != "Hello, World!" {
		t.Fatalf("unexpected output: %s", outStream.String())
	}
}

func TestMeaningfulStreamTransform(t *testing.T) {
	input := jsonExample
	t.Log("TestSimpleStream")
	tmpl := jsonTmpl
	inStream := NewJSONReader(bytes.NewBufferString(input))
	outStream := bytes.NewBuffer(nil)
	tfm, err := NewTemplateStreamTransformer(tmpl, inStream, outStream)
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}
	if err := tfm.Transform(); err != nil {
		t.Fatalf("failed to transform: %v", err)
	}
	outputStr := outStream.String()
	if outputStr != expectedJsonOutput {
		t.Fatalf("unexpected output: '%s' != '%s'", outputStr, expectedJsonOutput)
	}
}

func TestSimpleXMLStreamTransform(t *testing.T) {
	input := xmlExample
	t.Log("v")
	tmpl := xmlTmpl
	inStream := NewTextReader(bytes.NewBufferString(input))
	outStream := bytes.NewBuffer(nil)
	tfm, err := NewTemplateStreamTransformer(tmpl, inStream, outStream)
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}
	if err := tfm.Transform(); err != nil {
		t.Fatalf("failed to transform: %v", err)
	}
	outputStr := outStream.String()
	expected := `[{ "name": "Platypus"}]`
	if outputStr != expected {
		t.Fatalf("unexpected output: '%s' != '%s'", outputStr, expected)
	}
}

func TestMeaningfulXMLStreamTransform(t *testing.T) {
	input := xmlExample
	t.Log("TestMeaningfulXMLStreamTransform")
	tmpl := `
	{{- $s := separator ", " -}}
	[
	{{- $animals := getXPathAllOuter . "/root/animals/animal" -}}
	{{- range $animal := $animals -}}
	{{- call $s -}}
	{{- $animalName := getXPath $animal "/animal/name" -}}
	{{- $animalVotes := getXPath $animal "/animal/votes" -}}
	{"name": "{{ $animalName }}", "democratic_votes": {{ $animalVotes }}}
	{{- end -}}
	]`
	inStream := NewTextReader(bytes.NewBufferString(input))
	outStream := bytes.NewBuffer(nil)
	tfm, err := NewTemplateStreamTransformer(tmpl, inStream, outStream)
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}
	if err := tfm.Transform(); err != nil {
		t.Fatalf("failed to transform: %v", err)
	}
	outputStr := outStream.String()
	expected := `[{"name": "Platypus", "democratic_votes": 1}, {"name": "Quokka", "democratic_votes": 3}, {"name": "Quoll", "democratic_votes": 2}]`
	if outputStr != expected {
		t.Fatalf("unexpected output: '%s' != '%s'", outputStr, expected)
	}
}

func TestOpensslCertTextStreamTransform(t *testing.T) {
	input := openSSLCertTextOutput
	t.Log("TestOpensslCertTextStreamTransform")
	tmpl := `
	{{- $s := separator ", " -}}
	{{- $root := . -}}
	{{- $pubKeyAlgo := getRegexpFirstMatch $root "Public Key Algorithm: (?<anything>.*)" -}}
	{ "type": "x509", "public_key_algorithm": "{{ $pubKeyAlgo }}"}`
	inStream := NewTextReader(bytes.NewBufferString(input))
	outStream := bytes.NewBuffer(nil)
	tfm, err := NewTemplateStreamTransformer(tmpl, inStream, outStream)
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}
	if err := tfm.Transform(); err != nil {
		t.Fatalf("failed to transform: %v", err)
	}
	outputStr := outStream.String()
	expected := `{ "type": "x509", "public_key_algorithm": "rsaEncryption"}`
	if outputStr != expected {
		t.Fatalf("unexpected output: '%s' != '%s'", outputStr, expected)
	}
}
