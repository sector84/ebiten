// Copyright 2014 Hajime Hoshi
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build ignore

package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var license = ""

func init() {
	b, err := ioutil.ReadFile("../license.txt")
	if err != nil {
		panic(err)
	}
	license = string(b)

	// TODO: Year check
}

var copyright = ""

func init() {
	copyright = fmt.Sprintf("© %d Hajime Hoshi", time.Now().Year())
}

var stableVersion = ""

var devVersion = ""

func init() {
	b, err := ioutil.ReadFile("../version.txt")
	if err != nil {
		panic(err)
	}
	stableVersion = strings.TrimSpace(string(b))
}

func init() {
	b, err := exec.Command("git", "show", "master:version.txt").Output()
	if err != nil {
		panic(err)
	}
	devVersion = strings.TrimSpace(string(b))
}

func comment(text string) template.HTML {
	// TODO: text should be escaped
	return template.HTML("<!--\n" + text + "\n-->")
}

func safeHTML(text string) template.HTML {
	return template.HTML(text)
}

type example struct {
	Name string
}

func (e *example) ThumbWidth() int {
	if e.Name == "blocks" {
		return 256
	}
	return 320
}

func (e *example) ThumbHeight() int {
	if e.Name == "blocks" {
		return 240
	}
	return 240
}

func (e *example) Width() int {
	return e.ThumbWidth() * 2
}

func (e *example) Height() int {
	return e.ThumbHeight() * 2
}

func (e *example) Source() string {
	if e.Name == "blocks" {
		return "// Please read example/blocks/main.go and example/blocks/blocks/*.go"
	}

	path := filepath.Join(os.Getenv("GOPATH"), "src", "github.com", "hajimehoshi", "ebiten", "example", e.Name, "main.go")
	b, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	str := regexp.MustCompile("(?s)^.*?\n\n").ReplaceAllString(string(b), "")
	str = strings.Replace(str, "\t", "    ", -1)
	return str
}

func versions() string {
	return fmt.Sprintf("v%s (dev: v%s)", stableVersion, devVersion)
}

var examples = []example{
	{Name: "hue"},
	{Name: "keyboard"},
	{Name: "mosaic"},
	{Name: "perspective"},
	{Name: "rotate"},
	{Name: "blocks"},
}

func clear() error {
	if err := filepath.Walk("public", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Remove auto-generated html files.
		m, err := regexp.MatchString(".html$", path)
		if err != nil {
			return err
		}
		if !m {
			return nil
		}
		// Remove example resources that are copied.
		m, err = regexp.MatchString("public/example/images", path)
		if err != nil {
			return err
		}
		if !m {
			return nil
		}
		return os.Remove(path)
	}); err != nil {
		return err
	}
	return nil
}

func outputMain() error {
	f, err := os.Create("public/index.html")
	if err != nil {
		return err
	}
	defer f.Close()

	funcs := template.FuncMap{
		"comment":  comment,
		"safeHTML": safeHTML,
	}
	const templatePath = "index.tmpl.html"
	name := filepath.Base(templatePath)
	t, err := template.New(name).Funcs(funcs).ParseFiles(templatePath)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"License":       license,
		"Copyright":     copyright,
		"StableVersion": stableVersion,
		"DevVersion":    devVersion,
		"Examples":      examples,
	}
	return t.Funcs(funcs).Execute(f, data)
}

func outputExampleContent(e *example) error {
	const dir = "public/example"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, e.Name+".content.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	funcs := template.FuncMap{
		"comment":  comment,
		"safeHTML": safeHTML,
	}
	const templatePath = "examplecontent.tmpl.html"
	name := filepath.Base(templatePath)
	t, err := template.New(name).Funcs(funcs).ParseFiles(templatePath)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"License":   license,
		"Copyright": copyright,
		"Example":   e,
	}
	if err := t.Funcs(funcs).Execute(f, data); err != nil {
		return err
	}

	out := filepath.Join(dir, e.Name+".js")
	path := "github.com/hajimehoshi/ebiten/example/" + e.Name
	if err := exec.Command("gopherjs", "build", "-m", "-o", out, path).Run(); err != nil {
		return err
	}

	return nil
}

func outputExample(e *example) error {
	const dir = "public/example"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, e.Name+".html"))
	if err != nil {
		return err
	}
	defer f.Close()

	funcs := template.FuncMap{
		"comment":  comment,
		"safeHTML": safeHTML,
	}
	const templatePath = "example.tmpl.html"
	name := filepath.Base(templatePath)
	t, err := template.New(name).Funcs(funcs).ParseFiles(templatePath)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"License":   license,
		"Copyright": copyright,
		"Example":   e,
	}
	return t.Funcs(funcs).Execute(f, data)
}

func main() {
	if err := clear(); err != nil {
		log.Fatal(err)
	}
	if err := outputMain(); err != nil {
		log.Fatal(err)
	}
	for _, e := range examples {
		if err := outputExampleContent(&e); err != nil {
			log.Fatal(err)
		}
		if err := outputExample(&e); err != nil {
			log.Fatal(err)
		}
	}
}
