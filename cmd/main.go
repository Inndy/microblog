package main

import (
	"fmt"
	"html/template"
	"os"
	"path"
	"strings"

	"github.com/yuin/goldmark"
	gmAst "github.com/yuin/goldmark/ast"
	gmExtension "github.com/yuin/goldmark/extension"
	gmParser "github.com/yuin/goldmark/parser"
	gmText "github.com/yuin/goldmark/text"
)

const PUBLISH_DIR = "publish"
const ARTICLE_DIR = "article"
const DRAFT_DIR = "draft"

var html_template = `
{{define "header"}}<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>{{.Title}}</title>
	<meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
{{end}}


{{define "footer"}}
<hr>
<footer>
<p><small>Powered by <a href="https://github.com/Inndy/microblog">microblog</a></small></p>
</footer>
</body>
</html>
{{end}}

{{define "article"}}{{template "header"}}
<a href="#" onclick="history.back(); e.preventDefault();">back</a>
<article>
{{.Content}}
</article>
{{template "footer"}}
{{end}}

{{define "list"}}
<ul>
{{range .}}
	<li><a href="{{.Url}}">{{.Title}}</a></li>
{{end}}
</ul>
{{end}}
`

var templates *template.Template

var md = goldmark.New(
	goldmark.WithExtensions(gmExtension.GFM),
	goldmark.WithParserOptions(
		gmParser.WithAutoHeadingID(),
	),
)

func process_draft() {
	drafts, err := os.ReadDir(DRAFT_DIR)
	if err != nil {
		panic(fmt.Sprintf("can not read draft dir %q: %s", DRAFT_DIR, err))
	}

	for _, ent := range drafts {
		if !strings.HasSuffix(ent.Name(), ".md") {
			fmt.Printf("WARNING: skip non-markdown file in draft: %q", ent.Name())
			continue
		}

		info, err := ent.Info()
		if err != nil {
			fmt.Printf("ERROR: can not get file info %q: %s", ent.Name(), err)
			continue
		}


		mod_time := info.ModTime().UTC().Format("20060102-150405")
		new_name := fmt.Sprintf("%s--%s", mod_time, ent.Name())
		os.Rename(path.Join(DRAFT_DIR, ent.Name()), path.Join(ARTICLE_DIR, new_name))
	}
}

type base_template_data struct {
	Title string
}

type article_entry struct {
	Title string
	Url string
}

var article_list []*article_entry

func process_article() {
	articles, err := os.ReadDir(ARTICLE_DIR)
	if err != nil {
		panic(fmt.Sprintf("can not read article dir %q: %s", ARTICLE_DIR, err))
	}

	for _, ent := range articles {
		if !strings.HasSuffix(ent.Name(), ".md") {
			fmt.Printf("WARNING: skip non-markdown file in article: %q", ent.Name())
			continue
		}

		// TODO: check mtime, dont regenerate every file

		input_path := path.Join(ARTICLE_DIR, ent.Name())
		output_filename := strings.TrimSuffix(ent.Name(), ".md") + ".html"
		output_path := path.Join(PUBLISH_DIR, output_filename)
		title := process_file(input_path, output_path)

		fmt.Printf("generated %s\n", output_path)

		article_list = append(article_list, &article_entry{
			Title: title,
			Url: output_filename,
		})
	}
}

func find_first_title(node gmAst.Node) (ret gmAst.Node) {
	if node.Kind() == gmAst.KindHeading {
		return node
	}

	for ; node != nil; node = node.NextSibling() {
		if node.HasChildren() {
			if ret = find_first_title(node.FirstChild()); ret != nil {
				return
			}
		}

		if node.Kind() == gmAst.KindHeading {
			return node
		}
	}

	return
}

func process_file(input_path, output_path string) (title string) {
	content, err := os.ReadFile(input_path)
	if err != nil {
		fmt.Printf("ERROR: can not read input file %q: %s", input_path, err)
		return
	}

	fout, err := os.OpenFile(output_path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Printf("ERROR: can not open output file %q: %s", output_path, err)
		return
	}

	defer fout.Close()

	parsed_md := md.Parser().Parse(gmText.NewReader(content))
	if title_node := find_first_title(parsed_md); title_node != nil {
		title = string(title_node.Text(content))
	} else {
		title = strings.TrimSuffix(path.Base(input_path), ".md")
	}


	var buff strings.Builder
	md.Renderer().Render(&buff, content, parsed_md)

	templates.ExecuteTemplate(fout, "article", &struct{
		base_template_data
		Content template.HTML
	}{
		base_template_data{
			Title: title,
		},
		template.HTML(buff.String()),
	})

	return
}

func process_index() {
	output_path := path.Join(PUBLISH_DIR, "index.html")
	fout, err := os.OpenFile(output_path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Printf("ERROR: can not open output file %q: %s", output_path, err)
		return
	}

	// TODO: global site title
	template_data := base_template_data{
		Title: "microblog",
	}

	defer fout.Close()
	templates.ExecuteTemplate(fout, "header", &template_data)
	templates.ExecuteTemplate(fout, "list", article_list)
	templates.ExecuteTemplate(fout, "footer", &template_data)
}

func main() {
	os.Mkdir(PUBLISH_DIR, 0o755)
	os.Mkdir(ARTICLE_DIR, 0o755)
	os.Mkdir(DRAFT_DIR, 0o755)

	var err error
	templates, err = template.New("T").Parse(html_template)
	if err != nil {
		panic("can not parse header HTML template: " + err.Error())
	}

	process_draft()
	process_article()
	process_index()
}
