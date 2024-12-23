package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"
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

const form = `
<form id="input" autocomplete="off">
  <input type="hidden" name="puzzle" value="%s" />
  <input type="text" name="guess" value="" />
  <button type="submit">Guess</button>
</form>
`

func main() {
	foundPuzzles := generate()
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("dist")))
	mux.HandleFunc("POST /guess", func(writer http.ResponseWriter, request *http.Request) {
		puzzle := request.FormValue("puzzle")
		guess := request.FormValue("guess")
		if puzzle == "" || guess == "" {
			writer.WriteHeader(http.StatusBadRequest)
			fmt.Printf("Puzzle or guess is blank")
			return
		}
		index := slices.IndexFunc(foundPuzzles.Puzzles, func(puzz Puzzle) bool {
			return puzz.ID == puzzle
		})
		if index == -1 {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if slices.Contains(foundPuzzles.Puzzles[index].Metadata.Answers, guess) {
			writer.WriteHeader(http.StatusOK)
			return
		}
		writer.WriteHeader(http.StatusNotFound)
		return
	})
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

func generate() *Puzzles {
	foundPuzzles := getPuzzles()
	_ = os.RemoveAll("./dist")
	if err := os.MkdirAll("./dist", 0777); err != nil {
		log.Panic("Unable to create dist folder")
	}
	if err := os.WriteFile("dist/index.html", getIndexHTML(foundPuzzles.Index), 0644); err != nil {
		log.Panic(err)
	}
	_, _ = copyFile("layout/main.css", "dist/main.css")
	_, _ = copyFile("layout/main.js", "dist/main.js")
	for _, puzzle := range foundPuzzles.Puzzles {
		if err := os.MkdirAll("./dist/puzzles/"+puzzle.ID, 0777); err != nil {
			log.Panic("Unable to create puzzle folder")
		}
		if err := os.WriteFile("dist/puzzles/"+puzzle.ID+"/index.html", getPuzzleHTML(puzzle.ID, puzzle.Content), 0644); err != nil {
			log.Panic(err)
		}
		for _, file := range puzzle.Files {
			_, _ = copyFile("puzzles/"+puzzle.ID+"/"+file, "dist/puzzles/"+puzzle.ID+"/"+file)
		}
	}
	return foundPuzzles
}

func copyFile(src, dst string) (int64, error) {
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
	defer func() { _ = source.Close() }()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer func() { _ = destination.Close() }()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func getIndexHTML(content string) []byte {
	layoutBytes, err := os.ReadFile("./layout/index.html")
	if err != nil {
		log.Fatal(err)
	}
	return bytes.ReplaceAll(layoutBytes, []byte("<div id=\"puzzle\"></div>"), []byte(content))
}

func getPuzzleHTML(puzzle string, content string) []byte {
	layoutBytes, err := os.ReadFile("./layout/index.html")
	if err != nil {
		log.Fatal(err)
	}
	content = fmt.Sprintf("<div id=\"puzzle\">%s</div>", content)
	layoutBytes = bytes.ReplaceAll(layoutBytes, []byte("<div id=\"puzzle\"></div>"), []byte(content))
	return bytes.ReplaceAll(layoutBytes, []byte("<div id=\"input\"></div>"), []byte(fmt.Sprintf(form, puzzle)))
}

func getPuzzles() *Puzzles {
	var foundPuzzles = &Puzzles{}
	entries, err := os.ReadDir("./puzzles")
	if errors.Is(err, os.ErrNotExist) {
		log.Fatal("Puzzles folder must exist")
	}
	if err != nil {
		log.Fatal(err)
	}
	indexBytes, err := os.ReadFile("./puzzles/index.html")
	if errors.Is(err, os.ErrNotExist) {
		log.Fatal("puzzles/index.html - not found")
	}
	if err != nil {
		log.Fatal(err)
	}
	foundPuzzles.Index = string(indexBytes)
	for _, e := range entries {
		if e.IsDir() {
			foundPuzzles.Puzzles = append(foundPuzzles.Puzzles, *getPuzzle(e.Name()))
		}
	}
	return foundPuzzles
}

func getPuzzle(path string) *Puzzle {
	indexBytes, err := os.ReadFile("./puzzles/" + path + "/index.html")
	if errors.Is(err, os.ErrNotExist) {
		log.Fatal("puzzles/" + path + "/index.html - not found")
	}
	if err != nil {
		log.Fatal(err)
	}
	frontmatterBytes, contentBytes, err := splitFrontMatter(indexBytes)
	if err != nil {
		log.Fatal(err)
	}
	meta := &Puzzlemeta{}
	err = yaml.Unmarshal(frontmatterBytes, meta)
	if err != nil {
		log.Println("Unable to unmarshall frontmatter")
		log.Fatal(err)
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
		if !e.IsDir() && e.Name() != "index.html" {
			files = append(files, e.Name())
		}
	}
	return &Puzzle{
		ID:       path,
		Metadata: *meta,
		Content:  string(contentBytes),
		Files:    files,
	}
}

func splitFrontMatter(file []byte) ([]byte, []byte, error) {
	if !bytes.HasPrefix(file, []byte("<!--")) {
		return nil, nil, errors.New("no frontmatter")
	}
	index := bytes.Index(file, []byte("-->"))
	if index == -1 {
		return nil, nil, errors.New("no frontmatter")
	}
	return file[5:index], file[index+4:], nil
}
