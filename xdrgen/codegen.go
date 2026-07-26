package main

import (
	"bytes"
	"fmt"
	"go/format"
	"maps"
	"slices"
	"strings"
)

// Printer writes lines to a GeneratedFile with $variable substitution.
type Printer struct {
	f    *GeneratedFile
	vars []string // alternating old, new pairs for strings.NewReplacer
}

// Set updates or adds a variable binding. Returns the Printer for chaining.
func (p Printer) Set(name string, value any) Printer {
	key := "$" + name
	for i := 0; i < len(p.vars); i += 2 {
		if p.vars[i] == key {
			p.vars[i+1] = fmt.Sprint(value)
			return p
		}
	}
	p.vars = append(p.vars, key, fmt.Sprint(value))
	return p
}

// L prints a line with $variable substitution.
func (p Printer) L(s string) {
	if len(p.vars) > 0 {
		s = strings.NewReplacer(p.vars...).Replace(s)
	}
	p.f.P(s)
}

// Block writes a multi-line string with $variable substitution and auto-dedent.
// The common leading whitespace is stripped so the content can be indented to
// match the surrounding Go source without affecting the generated output.
func (p Printer) Block(s string) {
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	indent := ""
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimLeft(line, "\t ")
		lineIndent := line[:len(line)-len(trimmed)]
		if indent == "" || len(lineIndent) < len(indent) {
			indent = lineIndent
		}
	}
	r := strings.NewReplacer(p.vars...)
	for _, line := range lines {
		line = strings.TrimPrefix(line, indent)
		if len(p.vars) > 0 {
			line = r.Replace(line)
		}
		p.f.P(line)
	}
}

// GeneratedFile accumulates generated Go source with automatic import management.
type GeneratedFile struct {
	preamble    bytes.Buffer
	buf         bytes.Buffer
	packageName string
	imports     map[string]bool
}

// NewGeneratedFile creates a new generated file for the given package.
func NewGeneratedFile(packageName string) *GeneratedFile {
	return &GeneratedFile{
		packageName: packageName,
		imports:     make(map[string]bool),
	}
}

// Preamble writes a line before the package declaration (for lint directives, code-generated markers).
func (g *GeneratedFile) Preamble(v ...any) {
	for _, x := range v {
		fmt.Fprint(&g.preamble, x)
	}
	fmt.Fprintln(&g.preamble)
}

// P prints a line to the generated file. Arguments are concatenated.
func (g *GeneratedFile) P(v ...any) {
	for _, x := range v {
		fmt.Fprint(&g.buf, x)
	}
	fmt.Fprintln(&g.buf)
}

// Use returns a Printer that substitutes named $variables in each L() call.
// Variables are passed as alternating name/value pairs.
func (g *GeneratedFile) Use(vars ...any) Printer {
	p := Printer{f: g, vars: make([]string, 0, len(vars))}
	for i := 0; i+1 < len(vars); i += 2 {
		p.vars = append(p.vars, "$"+fmt.Sprint(vars[i]), fmt.Sprint(vars[i+1]))
	}
	return p
}

// AddImport registers an import path for inclusion in the generated file.
func (g *GeneratedFile) AddImport(importPath string) {
	g.imports[importPath] = true
}

// Content returns the formatted Go source with imports injected.
func (g *GeneratedFile) Content() ([]byte, error) {
	var importBuf bytes.Buffer
	importBuf.WriteString("package " + g.packageName + "\n\n")

	if len(g.imports) > 0 {
		importBuf.WriteString("import (\n")
		for _, importPath := range slices.Sorted(maps.Keys(g.imports)) {
			fmt.Fprintf(&importBuf, "\t%q\n", importPath)
		}
		importBuf.WriteString(")\n\n")
	}

	var full bytes.Buffer
	if g.preamble.Len() > 0 {
		full.Write(g.preamble.Bytes())
	}
	full.Write(importBuf.Bytes())
	full.Write(g.buf.Bytes())

	formatted, err := format.Source(full.Bytes())
	if err != nil {
		return full.Bytes(), fmt.Errorf("gofmt: %w (wrote %d bytes)", err, full.Len())
	}
	return formatted, nil
}

// Naming and string utilities.

// GoTypeName converts an XDR type name to Go (UpperCamelCase).
func GoTypeName(xdrName string) string {
	return snakeToCamel(xdrName)
}

func joinComma(ss []string) string { return strings.Join(ss, ", ") }

func snakeToCamel(s string) string {
	switch s {
	case "int":
		return "Int32"
	case "unsigned int":
		return "Uint32"
	case "hyper":
		return "Int64"
	case "unsigned hyper":
		return "Uint64"
	case "float":
		return "Float32"
	case "double":
		return "Float64"
	case "bool":
		return "Bool"
	}

	underscored := acronymSplit(s)
	lower := strings.ToLower(underscored)
	var result strings.Builder
	for part := range strings.SplitSeq(lower, "_") {
		if part == "" {
			continue
		}
		result.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return result.String()
}

// acronymSplit inserts underscores at acronym boundaries for CamelCase conversion.
// "SCPStatementType" → "SCP_Statement_Type", "AccountEntry" → "Account_Entry"
func acronymSplit(s string) string {
	var result strings.Builder
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				prev := s[i-1]
				if prev >= 'a' && prev <= 'z' || prev >= '0' && prev <= '9' {
					result.WriteByte('_')
				} else if prev >= 'A' && prev <= 'Z' && i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z' {
					result.WriteByte('_')
				}
			}
			result.WriteRune(c)
		} else {
			result.WriteRune(c)
		}
	}
	return result.String()
}
