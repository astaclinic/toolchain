package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/stoewer/go-strcase"
)

type Module struct {
	Package         string
	Name            string
	CapitalizedName string
}
type Config struct {
	Package string
	Name    string
	Modules []Module
}

func processFile(fileName string, config Config) error {
	tmplFile := filepath.Join("templates", fileName)
	tmpl, err := template.New(filepath.Base(fileName)).ParseFS(templateFS, tmplFile)
	fmt.Printf("[*] Processing %s\n", tmplFile)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}
	formattedSource := buf.Bytes()
	if strings.HasSuffix(fileName, ".go.tmpl") {
		formattedSource, err = format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("fail to format source %w", err)
		}
	}
	outFile := strings.ReplaceAll(filepath.Join(config.Name, strings.TrimSuffix(fileName, filepath.Ext(fileName))), "foo", config.Name)
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
	name := flag.String("name", "", "the project name")
	packageName := flag.String("package", "", "go package name of the project")
	modules := flag.String("modules", "", "fx modules needed for the project")
	flag.Parse()

	if *packageName == "" {
		panic("package name is required")
	}
	if *name == "" {
		nameString := path.Base(*packageName)
		name = &nameString
	}
	var moduleList []Module
	if *modules != "" {
		moduleStringList := strings.Split(*modules, ",")
		moduleList = make([]Module, len(moduleStringList))
		for i, moduleString := range moduleStringList {
			packageString := moduleString
			if moduleString == "postgres" {
				packageString = "db"
			}
			moduleList[i] = Module{
				Package:         fmt.Sprintf("%sfx", packageString),
				Name:            strcase.LowerCamelCase(moduleString),
				CapitalizedName: strcase.UpperCamelCase(moduleString),
			}
		}
	}

	config := Config{
		Package: *packageName,
		Name:    *name,
		Modules: moduleList,
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
		err := processFile(file, config)
		if err != nil {
			fmt.Printf("[!] failed to process file %s: %v\n", file, err)
		}
	}
}
