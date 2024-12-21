package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mdigger/goldmark-images"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/frontmatter"
)

type Puzzles struct {
	Index   string
	Puzzles []Puzzle
}
type Puzzle struct {
	ID       string
	Metadata Puzzlemeta
	Content  string
	Files    []string
}

type Puzzlemeta struct {
	Title   string   `yaml:"title"`
	Answers []string `yaml:"answers"`
	Hints   []string `yaml:"hints"`
}

func main() {
	generate()
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("dist")))
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", 8080),
		Handler: mux,
	}

	go func() {
		log.Printf("Listening on port %d", 8080)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
		log.Println("Stopped listening")
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Failed to shut down HTTP server: %v", err)
	}
}

func generate() {
	foundPuzzles := getPuzzles()
	_ = os.RemoveAll("./dist")
	if err := os.MkdirAll("./dist", 0777); err != nil {
		log.Panic("Unable to create dist folder")
	}
	if err := os.WriteFile("dist/index.html", getIndexHTML(foundPuzzles.Index), 0644); err != nil {
		log.Panic(err)
	}
	for _, puzzle := range foundPuzzles.Puzzles {
		if err := os.MkdirAll("./dist/puzzles/"+puzzle.ID, 0777); err != nil {
			log.Panic("Unable to create puzzle folder")
		}
		if err := os.WriteFile("dist/puzzles/"+puzzle.ID+"/index.html", getPuzzleHTML(puzzle.Content), 0644); err != nil {
			log.Panic(err)
		}
		for _, file := range puzzle.Files {
			_, _ = copy("puzzles/"+puzzle.ID+"/"+file, "dist/puzzles/"+puzzle.ID+"/"+file)
		}
	}
}

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func getIndexHTML(content string) []byte {
	layoutBytes, err := os.ReadFile("./layout/index.html")
	if err != nil {
		log.Fatal(err)
	}
	return bytes.ReplaceAll(layoutBytes, []byte("<slot />"), []byte(content))
}

func getPuzzleHTML(content string) []byte {
	layoutBytes, err := os.ReadFile("./layout/index.html")
	if err != nil {
		log.Fatal(err)
	}
	puzzleContent := append([]byte(content), []byte("<input type=text />")...)
	return bytes.ReplaceAll(layoutBytes, []byte("<slot />"), []byte(puzzleContent))
}

func getPuzzles() Puzzles {
	md := goldmark.New(
		goldmark.WithExtensions(&frontmatter.Extender{}),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithUnsafe(),
		),
	)
	var foundPuzzles = Puzzles{}
	entries, err := os.ReadDir("./puzzles")
	if errors.Is(err, os.ErrNotExist) {
		log.Fatal("Puzzles folder must exist")
	}
	if err != nil {
		log.Fatal(err)
	}
	indexBytes, err := os.ReadFile("./puzzles/index.md")
	if errors.Is(err, os.ErrNotExist) {
		log.Fatal("puzzles/index.md - not found")
	}
	if err != nil {
		log.Fatal(err)
	}
	var buf bytes.Buffer
	ctx := parser.NewContext()
	if err := md.Convert(indexBytes, &buf, parser.WithContext(ctx)); err != nil {
		log.Fatal("Unable to parse puzzle")
	}
	foundPuzzles.Index = buf.String()
	for _, e := range entries {
		if e.IsDir() {
			foundPuzzles.Puzzles = append(foundPuzzles.Puzzles, *getPuzzle(e.Name()))
		}
	}
	return foundPuzzles
}

func getPuzzle(path string) *Puzzle {
	indexBytes, err := os.ReadFile("./puzzles/" + path + "/index.md")
	if errors.Is(err, os.ErrNotExist) {
		log.Fatal("puzzles/" + path + "/index.md - not found")
	}
	if err != nil {
		log.Fatal(err)
	}
	imageURL := func(src string) string {
		return "/puzzles/" + path + "/" + src
	}
	md := goldmark.New(images.NewReplacer(imageURL),
		goldmark.WithExtensions(&frontmatter.Extender{}),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithUnsafe(),
		),
	)
	var buf bytes.Buffer
	ctx := parser.NewContext()
	if err := md.Convert(indexBytes, &buf, parser.WithContext(ctx)); err != nil {
		log.Fatal("Unable to parse puzzle")
	}
	d := frontmatter.Get(ctx)
	meta := &Puzzlemeta{}
	if err := d.Decode(&meta); err != nil {
		log.Fatal("Unable to parse puzzle metadata")
	}
	if meta.Title == "" {
		log.Fatal("Puzzle needs a title")
	}
	if len(meta.Answers) == 0 {
		log.Fatal("Puzzle needs at least one answer")
	}
	var files []string
	entries, err := os.ReadDir("./puzzles/" + path)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatal("Puzzles folder must exist")
	}
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "index.md" {
			files = append(files, e.Name())
		}
	}
	return &Puzzle{
		ID:       path,
		Metadata: *meta,
		Content:  buf.String(),
		Files:    files,
	}
}
