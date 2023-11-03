package clash

type Stack[T any] struct {
	data []T
}

func NewStack[T any]() *Stack[T] {
	return &Stack[T]{}
}

func (s *Stack[T]) Push(v T) {
	s.data = append(s.data, v)
}

func (s *Stack[T]) Pop() T {
	if len(s.data) == 0 {
		var v T
		return v
	}
	v := s.data[len(s.data)-1]
	s.data = s.data[:len(s.data)-1]
	return v
}

func (s *Stack[T]) PopData() []T {
	if len(s.data) == 0 {
		return nil
	}
	v := s.data
	s.data = nil
	return v
}

func (s *Stack[T]) Len() int {
	return len(s.data)
}
