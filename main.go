package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/csmith/envflag"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"text/template"
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
	Title   string              `yaml:"title"`
	Answers []string            `yaml:"answers"`
	Hints   []string            `yaml:"hints"`
	Unlocks map[string][]string `yaml:"unlocks"`
}

var (
	port  = flag.Int("port", 8080, "web server listen port")
	debug = flag.Bool("debug", false, "Enable debugging and disable caching")
)

func main() {
	envflag.Parse()
	foundPuzzles := getPuzzles()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /main.css", serveFile("layout/main.css"))
	mux.HandleFunc("GET /main.js", serveFile("layout/main.js"))
	mux.HandleFunc("GET /puzzles/{id}", addTrailingSlash)
	mux.HandleFunc("GET /puzzles/{id}/", servePuzzle(foundPuzzles))
	mux.HandleFunc("GET /puzzles/{id}/{file}", servePuzzleFile(foundPuzzles))
	mux.HandleFunc("GET /{$}", serveIndex(foundPuzzles))
	mux.HandleFunc("POST /guess", handleGuess(foundPuzzles))
	mux.HandleFunc("POST /hint", handleHint(foundPuzzles))
	var handler http.Handler
	if *debug {
		handler = DisableCaching(mux)
	} else {
		handler = mux
	}
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: handler,
	}

	go func() {
		log.Printf("Listening on port %d", *port)
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

func handleHint(foundPuzzles *Puzzles) func(http.ResponseWriter, *http.Request) {
	type HintRequest struct {
		Puzzle        string `json:"puzzle"`
		HintRequested int    `json:"hintRequested"`
	}
	type HintResponse struct {
		HintRequested int    `json:"hintRequested"`
		Hint          string `json:"hint"`
	}
	return func(writer http.ResponseWriter, request *http.Request) {
		hintRequest := HintRequest{}
		bodyBytes, err := io.ReadAll(request.Body)
		if err != nil {
			writer.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		err = json.Unmarshal(bodyBytes, &hintRequest)
		if err != nil {
			writer.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		puzzleID := hintRequest.Puzzle
		index := slices.IndexFunc(foundPuzzles.Puzzles, func(puzz Puzzle) bool {
			return puzz.ID == puzzleID
		})
		if index == -1 {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		hints := foundPuzzles.Puzzles[index].Metadata.Hints
		if hintRequest.HintRequested < 0 || hintRequest.HintRequested >= len(hints) {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		hint := hints[hintRequest.HintRequested]
		responseData, err := json.Marshal(HintResponse{
			HintRequested: hintRequest.HintRequested,
			Hint:          hint,
		})
		_, err = writer.Write(responseData)
		if err != nil {
			fmt.Printf("Error writing response: %s", err.Error())
		}
	}
}

func DisableCaching(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate;")
		w.Header().Set("pragma", "no-cache")
		next.ServeHTTP(w, r)
	})
}

func addTrailingSlash(writer http.ResponseWriter, request *http.Request) {
	http.Redirect(writer, request, request.URL.String()+"/", http.StatusTemporaryRedirect)
}

func servePuzzleFile(foundPuzzles *Puzzles) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		puzzleID := request.PathValue("id")
		index := slices.IndexFunc(foundPuzzles.Puzzles, func(puzz Puzzle) bool {
			return puzz.ID == puzzleID
		})
		if index == -1 {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		fileName := request.PathValue("file")
		if !slices.Contains(foundPuzzles.Puzzles[index].Files, fileName) {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		serveFile("puzzles/"+puzzleID+"/"+fileName)(writer, request)
	}
}

func serveFile(file string) func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		http.ServeFile(writer, request, file)
	}
}

func renderTemplate(templateName string, data interface{}, writer http.ResponseWriter) {
	templateBytes, err := os.ReadFile(templateName)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Printf("Unable to load template from disk: `%s`\n%s", templateName, err.Error())
		return
	}
	t := template.New("puzzle")
	t, err = t.Parse(string(templateBytes))
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Printf("Error parsing template: `%s`\n%s", templateName, err.Error())
		return
	}
	err = t.ExecuteTemplate(writer, "puzzle", data)
	if err != nil {
		fmt.Printf("Error executing template: `%s`\n%s", templateName, err.Error())
	}
}

func serveIndex(foundPuzzles *Puzzles) func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		renderTemplate("layout/index.html", foundPuzzles.Index, writer)
	}
}

func servePuzzle(foundPuzzles *Puzzles) func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		puzzleID := request.PathValue("id")
		index := slices.IndexFunc(foundPuzzles.Puzzles, func(puzz Puzzle) bool {
			return puzz.ID == puzzleID
		})
		if index == -1 {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		renderTemplate("layout/puzzle.html", foundPuzzles.Puzzles[index], writer)
	}
}

type GuessResult = string

const (
	guessCorrect   GuessResult = "correct"
	guessIncorrect GuessResult = "incorrect"
	guessUnlock    GuessResult = "unlock"
)

func handleGuess(foundPuzzles *Puzzles) func(writer http.ResponseWriter, request *http.Request) {
	type response struct {
		Guess  string      `json:"guess"`
		Result GuessResult `json:"result"`
		Unlock string      `json:"unlock,omitempty"`
	}

	return func(writer http.ResponseWriter, request *http.Request) {
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

		writer.Header().Add("Content-Type", "application/json")
		normalisedGuess := normaliseAnswer(guess)
		meta := foundPuzzles.Puzzles[index].Metadata
		if slices.Contains(meta.Answers, normalisedGuess) {
			_ = json.NewEncoder(writer).Encode(response{Guess: guess, Result: guessCorrect})
			return
		}
		for unlock := range meta.Unlocks {
			if slices.Contains(meta.Unlocks[unlock], normalisedGuess) {
				_ = json.NewEncoder(writer).Encode(response{Guess: guess, Result: guessUnlock, Unlock: unlock})
				return
			}
		}
		_ = json.NewEncoder(writer).Encode(response{Guess: guess, Result: guessIncorrect})
	}
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
	for i := range meta.Answers {
		meta.Answers[i] = normaliseAnswer(meta.Answers[i])
	}
	for k := range meta.Unlocks {
		for i := range meta.Unlocks[k] {
			meta.Unlocks[k][i] = normaliseAnswer(meta.Unlocks[k][i])
		}
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
	if !bytes.HasPrefix(file, []byte("<!--\n")) {
		return nil, nil, errors.New("no frontmatter")
	}
	index := bytes.Index(file, []byte("-->\n"))
	if index == -1 {
		return nil, nil, errors.New("no frontmatter")
	}
	return file[5:index], file[index+4:], nil
}

func normaliseAnswer(answer string) string {
	return strings.ToLower(strings.TrimSpace(answer))
}
