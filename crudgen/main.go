package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/stoewer/go-strcase"
)

type Field struct {
	Name            string `json:"name"`
	CapitalizedName string
	SnakeName       string
	LowerName       string
	Type            string `json:"type"`
	Optional        bool   `json:"optional"`
	Array           bool   `json:"array"`
	Association     string `json:"association"`
}

type Entity struct {
	Package               string
	Name                  string
	PluralName            string
	CapitalizedName       string
	CapitalizedPluralName string
	SnakeName             string
	SnakePluralName       string
	KebabName             string
	KebabPluralName       string
	LowerName             string
	Fields                []Field
}

func processFile(fileName string, entity Entity) error {
	tmplFile := filepath.Join("templates", fileName)
	tmpl, err := template.New(filepath.Base(fileName)).Funcs(template.FuncMap{
		"inc": func(i, j int) int {
			return i + j
		},
	}).ParseFS(templateFS, tmplFile)
	fmt.Printf("[*] Processing %s\n", tmplFile)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, entity); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}
	formattedSource := buf.Bytes()
	if strings.HasSuffix(fileName, ".go.tmpl") {
		formattedSource, err = format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("fail to format source %w", err)
		}
	}
	outFile := strings.ReplaceAll(filepath.Join("internal/pkg", entity.LowerName, strings.TrimSuffix(fileName, filepath.Ext(fileName))), "foo", entity.LowerName)
	fmt.Printf("[*] Writing to %s\n", outFile)
	err = os.MkdirAll(filepath.Dir(outFile), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	f, err := os.OpenFile(outFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()
	f.Write(formattedSource)
	return nil
}

//go:embed templates
var templateFS embed.FS

func main() {
	name := flag.String("name", "", "name of the entity")
	pluralName := flag.String("plural", "", "plural name of the entity")
	packageName := flag.String("package", "", "go package name of the entity")
	fieldsJson := flag.String("fields", "fields.json", "fields of the entity")
	flag.Parse()
	if *name == "" {
		panic("name is required")
	}
	if *pluralName == "" {
		pluralNameString := *name + "s"
		pluralName = &pluralNameString
	}
	if *packageName == "" {
		packageNameString := fmt.Sprintf("github.com/astaclinic/core/%s", strcase.KebabCase(*name))
		packageName = &packageNameString
	}

	var fields []Field

	fieldsJsonf, err := os.Open(*fieldsJson)
	if err != nil {
		fmt.Printf("[!] failed to open fields json: %v\n", err)
	}
	defer fieldsJsonf.Close()
	fieldsJsonb, err := io.ReadAll(fieldsJsonf)
	if err != nil {
		fmt.Printf("[!] failed to read fields json: %v\n", err)
	}
	if err := json.Unmarshal(fieldsJsonb, &fields); err != nil {
		fmt.Printf("[!] failed to unmarshal fields json: %v\n", err)
	}

	for i, field := range fields {
		fields[i].CapitalizedName = strcase.UpperCamelCase(field.Name)
		fields[i].SnakeName = strcase.SnakeCase(field.Name)
		fields[i].LowerName = strings.ToLower(field.Name)
	}

	entity := Entity{
		Package:               *packageName,
		Name:                  strcase.LowerCamelCase(*name),
		PluralName:            strcase.LowerCamelCase(*pluralName),
		CapitalizedName:       strcase.UpperCamelCase(*name),
		CapitalizedPluralName: strcase.UpperCamelCase(*pluralName),
		SnakeName:             strcase.SnakeCase(*name),
		SnakePluralName:       strcase.SnakeCase(*pluralName),
		KebabName:             strcase.KebabCase(*name),
		KebabPluralName:       strcase.KebabCase(*pluralName),
		LowerName:             strings.ToLower(*name),
		Fields:                fields,
	}

	var tmplList []string
	if err := fs.WalkDir(templateFS, "templates", func(path string, d os.DirEntry, err error) error {
		if filepath.Ext(path) == ".tmpl" {
			tmplFile, err := filepath.Rel("templates", path)
			if err != nil {
				return fs.SkipDir
			}
			tmplList = append(tmplList, tmplFile)
		}
		return nil
	}); err != nil {
		fmt.Printf("[!] failed to list files: %v\n", err)
	}

	for _, file := range tmplList {
		err := processFile(file, entity)
		if err != nil {
			fmt.Printf("[!] failed to process file %s: %v\n", file, err)
		}
	}
}
