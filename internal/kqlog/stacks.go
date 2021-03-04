package kqlog

// Provide a couple simple stack types. They panic if peeking or popping from
// an empty stack.

type boolStack []bool

func (s *boolStack) Push(v bool) {
	*s = append(*s, v)
}

func (s *boolStack) Pop() bool {
	l := len(*s)
	if l == 0 {
		panic("empty stack")
	}
	v := (*s)[l-1]
	*s = (*s)[:l-1]
	return v
}

func (s *boolStack) Peek() bool {
	l := len(*s)
	if l == 0 {
		panic("empty stack")
	}
	return (*s)[l-1]
}

func (s *boolStack) Len() int {
	return len(*s)
}

type tokenStack []token

func (s *tokenStack) Push(v token) {
	*s = append(*s, v)
}

func (s *tokenStack) Pop() token {
	l := len(*s)
	if l == 0 {
		panic("empty stack")
	}
	v := (*s)[l-1]
	*s = (*s)[:l-1]
	return v
}

func (s *tokenStack) Peek() token {
	l := len(*s)
	if l == 0 {
		panic("empty stack")
	}
	return (*s)[l-1]
}

func (s *tokenStack) Len() int {
	return len(*s)
}
