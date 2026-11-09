package nfse

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfflineSchemaValidatorWithSyntheticSchema(t *testing.T) {
	if _, e := exec.LookPath("xmllint"); e != nil {
		t.Skip("requires local xmllint; no official package is implied")
	}
	dir := t.TempDir()
	schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="` + Namespace + `" xmlns="` + Namespace + `" elementFormDefault="qualified"><xs:element name="DPS"><xs:complexType><xs:sequence><xs:element name="infDPS" type="xs:anyType"/></xs:sequence><xs:attribute name="versao" type="xs:string" use="required" fixed="1.01"/></xs:complexType></xs:element></xs:schema>`)
	path := filepath.Join(dir, "synthetic-only.xsd")
	if e := os.WriteFile(path, schema, 0o600); e != nil {
		t.Fatal(e)
	}
	set := SchemaSet{Directory: dir, Version: "1.01", DPSFile: "synthetic-only.xsd", NFSeFile: "synthetic-only.xsd", Files: map[string]string{"synthetic-only.xsd": sha(schema)}}
	validator, e := NewSchemaValidator(set)
	if e != nil {
		t.Fatal(e)
	}
	raw := []byte(`<DPS xmlns="` + Namespace + `" versao="1.01"><infDPS Id="` + testDPSID() + `"/></DPS>`)
	if e = validator.Validate(context.Background(), "dps", "1.01", raw); e != nil {
		t.Fatal("local XSD validation failed", e)
	}
	if e = validator.Validate(context.Background(), "dps", "1.01", []byte(strings.Replace(string(raw), `versao="1.01"`, `versao="0"`, 1))); e == nil {
		t.Fatal("schema violation ignored")
	}
	if e = os.WriteFile(path, append(schema, []byte("changed")...), 0o600); e != nil {
		t.Fatal(e)
	}
	if e = validator.Validate(context.Background(), "dps", "1.01", raw); e == nil {
		t.Fatal("changed schema checksum ignored")
	}
	entity := []byte(`<?xml version="1.0"?><!DOCTYPE x [<!ENTITY e SYSTEM "file:///etc/passwd">]><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	if e = os.WriteFile(path, entity, 0o600); e != nil {
		t.Fatal(e)
	}
	set.Files["synthetic-only.xsd"] = sha(entity)
	if _, e = NewSchemaValidator(set); e == nil {
		t.Fatal("unpinned DOCTYPE permitted")
	}
	external := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="file:///etc/passwd"/></xs:schema>`)
	if e = os.WriteFile(path, external, 0o600); e != nil {
		t.Fatal(e)
	}
	set.Files["synthetic-only.xsd"] = sha(external)
	if _, e = NewSchemaValidator(set); e == nil {
		t.Fatal("external schema import permitted")
	}
}
