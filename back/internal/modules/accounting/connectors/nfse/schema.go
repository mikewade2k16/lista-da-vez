package nfse

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SchemaSet describes an offline, reviewed package, not a remote schema URL.
// No package is shipped/claimed official until its original bytes are available.
type SchemaSet struct {
	Directory, Version, DPSFile, NFSeFile string
	Files                                 map[string]string
}
type SchemaValidator struct {
	set           SchemaSet
	root, program string
}

func NewSchemaValidator(set SchemaSet) (*SchemaValidator, error) {
	if set.Version != "1.01" || len(set.Files) == 0 || len(set.Files) > 100 {
		return nil, failure("schema_package_unavailable")
	}
	root, e := filepath.EvalSymlinks(set.Directory)
	if e != nil {
		return nil, failure("schema_package_unavailable")
	}
	root, e = filepath.Abs(root)
	if e != nil {
		return nil, failure("schema_package_unavailable")
	}
	program, e := exec.LookPath("xmllint")
	if e != nil {
		return nil, failure("schema_validator_unavailable")
	}
	copySet := set
	copySet.Files = map[string]string{}
	for path, hash := range set.Files {
		copySet.Files[path] = hash
	}
	v := &SchemaValidator{set: copySet, root: root, program: program}
	if _, ok := set.Files[set.DPSFile]; !ok {
		return nil, failure("schema_root_missing")
	}
	if _, ok := set.Files[set.NFSeFile]; !ok {
		return nil, failure("schema_root_missing")
	}
	if e = v.verifyPackage(); e != nil {
		return nil, e
	}
	return v, nil
}

// officialSchemaDirectives pins DOCTYPE directives by SHA-256 of their bytes.
var officialSchemaDirectives = map[string]bool{
	// xmldsig-core-schema.xsd, NFSe-ESQUEMAS_XSD-v1.01-20260209 (internal entities only).
	"36e8d56299b08d34d5f9113c5546ea8b1b64e076cedae18274acf4c329642d96": true,
}

func (v *SchemaValidator) verifyPackage() error {
	total := 0
	for name, checksum := range v.set.Files {
		if name == "" || filepath.IsAbs(name) || filepath.Clean(name) != name || strings.HasPrefix(name, "..") || !checksumPattern.MatchString(checksum) {
			return failure("schema_path_invalid")
		}
		path, e := filepath.EvalSymlinks(filepath.Join(v.root, name))
		if e != nil {
			return failure("schema_package_unavailable")
		}
		relative, e := filepath.Rel(v.root, path)
		if e != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return failure("schema_path_invalid")
		}
		f, e := os.Open(path)
		if e != nil {
			return failure("schema_package_unavailable")
		}
		raw, e := io.ReadAll(io.LimitReader(f, 2*1024*1024+1))
		closeError := f.Close()
		if e != nil || closeError != nil || len(raw) > 2*1024*1024 {
			return failure("schema_package_invalid")
		}
		total += len(raw)
		if total > 12*1024*1024 || sha(raw) != checksum {
			return failure("schema_hash_mismatch")
		}
		decoder := xml.NewDecoder(bytes.NewReader(raw))
		for {
			token, e := decoder.Token()
			if e == io.EOF {
				break
			}
			if e != nil {
				return failure("schema_package_invalid")
			}
			switch item := token.(type) {
			case xml.Directive:
				// Only the exact W3C xmldsig DOCTYPE shipped in the official package is
				// accepted; xmllint runs with --nonet, so its external DTD is never fetched.
				if !officialSchemaDirectives[sha(item)] {
					return failure("schema_directive_forbidden")
				}
			case xml.StartElement:
				for _, a := range item.Attr {
					if a.Name.Local != "schemaLocation" {
						continue
					}
					if strings.Contains(a.Value, ":") || filepath.IsAbs(a.Value) {
						return failure("schema_external_reference_forbidden")
					}
					target := filepath.Clean(filepath.Join(filepath.Dir(name), a.Value))
					if _, ok := v.set.Files[target]; !ok {
						return failure("schema_external_reference_forbidden")
					}
				}
			}
		}
	}
	return nil
}
func (v *SchemaValidator) Validate(ctx context.Context, kind, version string, raw []byte) error {
	if v == nil || version != v.set.Version {
		return failure("schema_version_unavailable")
	}
	projection, e := inspectXML(raw)
	if e != nil {
		return e
	}
	file := v.set.DPSFile
	switch kind {
	case "dps":
		if projection.Root != "DPS" {
			return failure("schema_root_invalid")
		}
	case "nfse":
		file = v.set.NFSeFile
		if projection.Root != "NFSe" {
			return failure("schema_root_invalid")
		}
	default:
		return failure("schema_kind_invalid")
	}
	if e = v.verifyPackage(); e != nil {
		return e
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, v.program, "--nonet", "--noout", "--schema", filepath.Join(v.root, file), "-") //nolint:gosec // Fixed executable and reviewed, contained schema path; XML only on stdin.
	command.Stdin = bytes.NewReader(raw)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	command.Env = []string{"LC_ALL=C", "XML_CATALOG_FILES="}
	if e = command.Run(); e != nil {
		return failure("xml_schema_invalid")
	}
	return nil
}
