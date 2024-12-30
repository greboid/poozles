package main

type Poozles struct {
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

type Guess struct {
	Puzzle string `json:"puzzle"`
	Guess  string `json:"guess"`
}
type GuessResponse struct {
	Puzzle      string      `json:"puzzle"`
	Guess       string      `json:"guess"`
	Result      GuessResult `json:"result"`
	Unlock      string      `json:"unlock,omitempty"`
	Replacement string      `json:"replacement,omitempty"`
}

type HintRequest struct {
	Puzzle        string `json:"puzzle"`
	HintRequested int    `json:"hintRequested"`
}
type HintResponse struct {
	HintRequested int    `json:"hintRequested"`
	Hint          string `json:"hint"`
}
