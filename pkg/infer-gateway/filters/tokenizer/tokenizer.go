package tokenizer

type Tokenizer interface {
	CalculateTokenNum(string) (int, error)
}
