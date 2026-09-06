package link

type Link interface {
	Chat(input string, model string) ([]byte, error)
}
