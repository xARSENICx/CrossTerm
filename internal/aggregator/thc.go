package aggregator

func init() {
	Register(Aggregator{
		Name:       "The Hindu Crossword",
		ScriptDir:  "aggregators/thc-aggregator",
		EntryPoint: "thc-puz-aggregator.py",
		OutputDir:  "data/puzzles",
		InputLabel: "Enter date (DD/MM/YYYY):",
		Args: func(input string) []string {
			return []string{input}
		},
	})
}
