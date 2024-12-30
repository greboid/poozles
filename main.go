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
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"text/template"
	"time"
)

var (
	port  = flag.Int("port", 8080, "web server listen port")
	debug = flag.Bool("debug", true, "Enable debugging and disable caching")
)

func main() {
	envflag.Parse()
	if *debug {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		})))
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})))
	}
	foundPuzzles := getPuzzles()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /main.css", serveFile("layout/main.css"))
	mux.HandleFunc("GET /main.js", serveFile("layout/main.js"))
	mux.HandleFunc("GET /puzzles/{id}", addTrailingSlash)
	mux.HandleFunc("GET /puzzles/{id}/{$}", servePuzzle(foundPuzzles))
	mux.HandleFunc("GET /puzzles/{id}/{file}", servePuzzleFile(foundPuzzles))
	mux.HandleFunc("GET /{$}", serveIndex(foundPuzzles))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("POST /guess", handleGuess(foundPuzzles))
	mux.HandleFunc("POST /hint", handleHint(foundPuzzles))

	var handler http.Handler
	if *debug {
		handler = NotFoundHandler(DisableCaching(mux))
	} else {
		handler = NotFoundHandler(mux)
	}
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: handler,
	}

	go func() {
		slog.Info("Starting server", "port", *port)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server error", "error", err)
			os.Exit(2)
		}
		slog.Info("Stopped server")
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Failed to shut down HTTP server", "error", err)
		os.Exit(2)
	}
}

func handleHint(foundPuzzles *Poozles) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		hintRequest := HintRequest{}
		bodyBytes, err := io.ReadAll(request.Body)
		if err != nil {
			writer.WriteHeader(http.StatusUnprocessableEntity)
			slog.Debug("Empty body when requesting hint")
			return
		}
		err = json.Unmarshal(bodyBytes, &hintRequest)
		if err != nil {
			writer.WriteHeader(http.StatusUnprocessableEntity)
			slog.Debug("Unable to unmarshall the body into a HintRequest")
			return
		}
		puzzleID := hintRequest.Puzzle
		index := slices.IndexFunc(foundPuzzles.Puzzles, func(puzz Puzzle) bool {
			return puzz.ID == puzzleID
		})
		if index == -1 {
			writer.WriteHeader(http.StatusNotFound)
			slog.Debug("Puzzle ID not found", "puzzleID", hintRequest.Puzzle)
			return
		}
		hints := foundPuzzles.Puzzles[index].Metadata.Hints
		if hintRequest.HintRequested < 0 || hintRequest.HintRequested >= len(hints) {
			writer.WriteHeader(http.StatusBadRequest)
			slog.Debug("Hint index not found", "puzzleID", hintRequest.Puzzle, "hintIndex", hintRequest.HintRequested)
			return
		}
		hint := hints[hintRequest.HintRequested]
		hintResponse := &HintResponse{
			HintRequested: hintRequest.HintRequested,
			Hint:          hint,
		}
		responseData, err := json.Marshal(hintResponse)
		_, err = writer.Write(responseData)
		if err != nil {
			slog.Error("Error writing response to client", "error", err.Error())
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

func servePuzzleFile(foundPuzzles *Poozles) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		puzzleID := request.PathValue("id")
		index := slices.IndexFunc(foundPuzzles.Puzzles, func(puzz Puzzle) bool {
			return puzz.ID == puzzleID
		})
		if index == -1 {
			writer.WriteHeader(http.StatusNotFound)
			slog.Debug("Puzzle ID not found", "puzzleID", puzzleID)
			return
		}
		fileName := request.PathValue("file")
		if !slices.Contains(foundPuzzles.Puzzles[index].Files, fileName) {
			writer.WriteHeader(http.StatusNotFound)
			slog.Debug("Puzzle file not found", "puzzleID", puzzleID, "file", fileName)
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
		slog.Error("Unable to load template from disk", "template", templateName, "error", err)
		return
	}
	t := template.New("puzzle")
	t, err = t.Parse(string(templateBytes))
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		slog.Error("Error parsing template", "template", templateName, "error", err)
		return
	}
	err = t.ExecuteTemplate(writer, "puzzle", data)
	if err != nil {
		slog.Error("Error executing template", "template", templateName, "error", err)
	}
}

func serveIndex(foundPuzzles *Poozles) func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		renderTemplate("layout/index.html", foundPuzzles.Index, writer)
	}
}

func servePuzzle(foundPuzzles *Poozles) func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		puzzleID := request.PathValue("id")
		index := slices.IndexFunc(foundPuzzles.Puzzles, func(puzz Puzzle) bool {
			return puzz.ID == puzzleID
		})
		if index == -1 {
			writer.WriteHeader(http.StatusNotFound)
			slog.Debug("Puzzle ID not found", "puzzleID", puzzleID)
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

func handleGuess(foundPuzzles *Poozles) func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		guess := Guess{}
		bodyBytes, err := io.ReadAll(request.Body)
		if err != nil {
			writer.WriteHeader(http.StatusUnprocessableEntity)
			slog.Debug("Empty body when submitting guest")
			return
		}
		err = json.Unmarshal(bodyBytes, &guess)
		if err != nil {
			writer.WriteHeader(http.StatusUnprocessableEntity)
			slog.Debug("Unable to unmarshall the body into a Guess")
			return
		}
		if guess.Puzzle == "" || guess.Guess == "" {
			writer.WriteHeader(http.StatusBadRequest)
			slog.Debug("Puzzle or guess is blank", "puzzleID", guess.Puzzle, "guess", guess.Guess)
			return
		}
		index := slices.IndexFunc(foundPuzzles.Puzzles, func(puzz Puzzle) bool {
			return puzz.ID == guess.Puzzle
		})
		if index == -1 {
			writer.WriteHeader(http.StatusBadRequest)
			slog.Debug("Puzzle ID not found", "puzzleID", guess.Puzzle)
			return
		}

		writer.Header().Add("Content-Type", "application/json")
		normalisedGuess := normaliseAnswer(guess.Guess)
		meta := foundPuzzles.Puzzles[index].Metadata
		if slices.Contains(meta.Answers, normalisedGuess) {
			guessResponse := &GuessResponse{Puzzle: foundPuzzles.Puzzles[index].ID, Guess: guess.Guess, Result: guessCorrect}
			successBytes, err := os.ReadFile("puzzles/" + guess.Puzzle + "/success.html")
			if err == nil {
				guessResponse.Replacement = string(successBytes)
			}
			_ = json.NewEncoder(writer).Encode(guessResponse)
			slog.Debug("Correct guess", "puzzleID", guess.Puzzle, "guess", guess.Guess, "normlisedGuess", normalisedGuess)
			return
		}
		for unlock := range meta.Unlocks {
			if slices.Contains(meta.Unlocks[unlock], normalisedGuess) {
				guessResponse := &GuessResponse{Puzzle: foundPuzzles.Puzzles[index].ID, Guess: guess.Guess, Result: guessUnlock, Unlock: unlock}
				_ = json.NewEncoder(writer).Encode(guessResponse)
				slog.Debug("Unlock guess", "puzzleID", guess.Puzzle, "guess", guess.Guess, "normlisedGuess", normalisedGuess)
				return
			}
		}
		guessResponse := &GuessResponse{Puzzle: foundPuzzles.Puzzles[index].ID, Guess: guess.Guess, Result: guessIncorrect}
		_ = json.NewEncoder(writer).Encode(guessResponse)
		slog.Debug("Incorrect guess", "puzzleID", guess.Puzzle, "guess", guess.Guess, "normlisedGuess", normalisedGuess)
	}
}

func getPuzzles() *Poozles {
	var foundPuzzles = &Poozles{}
	entries, err := os.ReadDir("./puzzles")
	if errors.Is(err, os.ErrNotExist) {
		slog.Error("Poozles folder must exist", "error", err)
		os.Exit(3)
	}
	if err != nil {
		slog.Error("Error reading puzzles folder", "error", err)
		os.Exit(3)
	}
	indexBytes, err := os.ReadFile("./puzzles/index.html")
	if errors.Is(err, os.ErrNotExist) {
		slog.Error("Poozles folder needs to contain index.html", "error", err)
		os.Exit(3)
	}
	if err != nil {
		slog.Error("Error reading puzzles index", "error", err)
		os.Exit(3)
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
		slog.Error("Each puzzle needs to have an index.html", "error", err, "puzzle", path)
		os.Exit(4)
	}
	if err != nil {
		slog.Error("Error loading the puzzle index.html", "error", err, "puzzle", path)
		os.Exit(4)
	}
	frontmatterBytes, contentBytes, err := splitFrontMatter(indexBytes)
	if err != nil {
		slog.Error("Error loading the puzzle frontmatter", "error", err, "puzzle", path)
		os.Exit(4)
	}
	meta := &Puzzlemeta{}
	err = yaml.Unmarshal(frontmatterBytes, meta)
	if err != nil {
		slog.Error("Error unmarshalling frontmatter", "error", err, "puzzle", path)
		os.Exit(4)
	}
	if meta.Title == "" {
		slog.Error("The `title` attribute is required", "error", err, "puzzle", path)
		os.Exit(4)
	}
	if len(meta.Answers) == 0 {
		slog.Error("The `answers` attribute must have at least 1 answer", "error", err, "puzzle", path)
		os.Exit(4)
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
		slog.Error("Poozles folder must exist", "error", err)
		os.Exit(4)
	}
	if err != nil {
		slog.Error("Error reading puzzles folder", "error", err)
		os.Exit(4)
	}
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "index.html" && e.Name() != "success.html" {
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
