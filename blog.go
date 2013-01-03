package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/russross/blackfriday"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"text/template"
)

var (
	codeBlockRE     = *regexp.MustCompile("(?smU)({codeblock})")
	endBlockRE      = *regexp.MustCompile("(?smU)({endcode})")
	BLOGPATH        string
	MARKDOWNPATH    string
	templateMap     = make(map[string]*template.Template)
	DEFAULTTEMPLATE string
	d_flag          = flag.String("d", "index", "-d <templatename>")
	h_flag          = flag.Bool("help", false, "use -help to display program options")
	w_flag          = flag.Bool("w", false, "use -w to suppress warnings")
	c_flag          = flag.Bool("c", false, `use -c to expand 
		{codeblock}(.*){endblock} into highlighted blocks`)
)

const (
	PATHSEP       = string(os.PathSeparator)
)

type Post struct {
	Rootpath          string
	ProcessedMarkdown string
}

/* change {codeblock}(.*){endcode} to
   <pre class="prettyprint linenums:1">(.*)</pre> */
func setCodeBlocks(markdown string) string {
	firstPass := codeBlockRE.ReplaceAllString(markdown,
		"<pre class=\"prettyprint linenums:1\">")
	secondPass := endBlockRE.ReplaceAllString(firstPass, "</pre>")
	return secondPass
}

/* replaces {disqus} with the appropriate javascript to
   include a comments section */
func embedDisqus(fileName, markdown string) string {
	relativeToBlogpath := strings.Replace(fileName, MARKDOWNPATH, "", -1)
	relativeToBlogpath = strings.Replace(relativeToBlogpath, ".md", ".html", -1)
	disqusTemplate, err := ioutil.ReadFile(BLOGPATH + "/disqus.templ")
	if err != nil {
		panic(err.Error())
	}
	templ := template.Must(template.New("disqus template").Parse(string(disqusTemplate)))
	b := new(bytes.Buffer)
	err = templ.Execute(b, relativeToBlogpath)
	if err != nil {
		panic(err.Error())
	}
	markdown = strings.Replace(markdown, "{disqus}", b.String(), -1)
	return markdown
}

/* processes the markdown contained in file called fileName.  If
   containsCode is true then {codeblock}(.*){endcode} will be
   processed according to setCodeBlocks above */
func processMarkdown(fileName string, containsCode bool) string {
	/* file is an []byte */
	file, err := ioutil.ReadFile(fileName)
	if err != nil {
		panic(err.Error())
	}
	markdown := string(blackfriday.MarkdownCommon(file))
	if containsCode {
		markdown = setCodeBlocks(markdown)
	}
	markdown = embedDisqus(fileName, markdown)
	return markdown
}

/* returns a pointer to a Template that is the result
   of reading in the contents of file templateName */
func readTemplate(templateName string) *template.Template {
	contents, err := ioutil.ReadFile(templateName)
	if err != nil {
		panic(err.Error())
	}
	template := template.Must(template.New("blog template").Parse(string(contents)))
	return template
}

/* outputs a page of html text to file outFile derived from
   executing templ based on ProcessedMarkdown */
func outputPage(templ *template.Template, ProcessedMarkdown, outFile string) error {
	relativeToBlogpath := strings.Replace(outFile, BLOGPATH+PATHSEP, "", -1)
	pathElements := strings.Split(relativeToBlogpath, PATHSEP)
	pathBuffer := new(bytes.Buffer)
	for i := 0; i < len(pathElements)-1; i++ {
		pathBuffer.WriteString(".." + PATHSEP)
	}
	b := new(bytes.Buffer)
	post := Post{pathBuffer.String(), ProcessedMarkdown}
	err := templ.Execute(b, post)
	if err != nil {
		panic(err.Error())
	}
	output := []byte(b.String())
	err = ioutil.WriteFile(outFile, output, 0666)
	return err
}

/* opens index.html to give a nice preview of the site.  This
   may not work if user is not using a mac, since the open
   command is not always built-in.  */
func sitePreview() error {
	osType := runtime.GOOS
	var instruction string
	if osType == "darwin" {
		instruction = "open"
	} else if osType == "linux" {
		instruction = "xdg-open"
	} else {
		if !*w_flag {
			fmt.Println("sitePreview not supported with os type ", osType)
		}
		return nil

	}
	open := exec.Command(instruction, BLOGPATH+PATHSEP+"index.html")
	err := open.Run()
	return err
}

/* builds the blog by calling buildFunc on each of the markdown files
   found in BLOGPATH/markdown */
func buildBlog() {
	buildFunc := func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && filepath.Ext(path) == ".md" {
			markdown := processMarkdown(path, *c_flag)
			fileName :=
				path[strings.LastIndex(path, PATHSEP)+1 : strings.LastIndex(path, ".")]
			templ, ok := templateMap[fileName+".md"]
			if !ok {
				templ = templateMap[DEFAULTTEMPLATE+".md"]
			}
			/* target is the path relative to BLOGPATH of the output file */
			target := strings.Replace(path, MARKDOWNPATH+PATHSEP, "", -1)
			/* rootTest determines if this is going to be a subdirectory of BLOGPATH */
			outDir := ""
			rootTest := strings.LastIndex(target, PATHSEP)
			if rootTest != -1 {
				/* target is a directory */
				outDir = target[:strings.LastIndex(target, PATHSEP)]
				/* make the directory and all necessary children */
				os.MkdirAll(outDir, 0755)
			}
			if outDir == "" {
				err = outputPage(templ, markdown,
					BLOGPATH+PATHSEP+fileName+".html")
			} else {
				err = outputPage(templ, markdown,
					BLOGPATH+PATHSEP+outDir+PATHSEP+fileName+".html")
			}

		}
		if err != nil {
			panic(err.Error())
		}

		return err
	}
	filepath.Walk(MARKDOWNPATH, buildFunc)
}

/* traverses the templates directory and registers templates.  For
   each .templ file x this function finds, it will map x.md to the
   template pointer it gets from calling readTemplate on x */
func registerTemplates() {
	templFunc := func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && filepath.Ext(path) == ".templ" {
			fileName :=
				path[strings.LastIndex(path, PATHSEP)+1 : strings.LastIndex(path, ".")]
			templ := readTemplate(path)
			templateMap[fileName+".md"] = templ
		}
		if err != nil {
			panic(err.Error())
		}
		return err
	}
	filepath.Walk(BLOGPATH+PATHSEP+"templates", templFunc)
}

func main() {
	blogpath := os.Getenv("BLOGPATH")
	if blogpath == "" {
		fmt.Println("error: BLOGPATH environment variable not configured")
		os.Exit(-1)
	} else {
		BLOGPATH = blogpath
	}
	MARKDOWNPATH = BLOGPATH + PATHSEP + "markdown"
	flag.Parse()
	if *h_flag {
		flag.Usage()
		os.Exit(1)
	}
	DEFAULTTEMPLATE = *d_flag
	registerTemplates()
	buildBlog()
	err := sitePreview()
	if err != nil {
		panic(err.Error())
	}
}