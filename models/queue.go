package models

type Queue struct {
	Elements []interface{}
}

func (q *Queue) Push(element interface{}) int {
	q.Elements = append(q.Elements, element)
	return q.GetLength()
}

func (q *Queue) Pop() {

	if !q.IsEmpty() {
		q.Elements = append(q.Elements[:0], q.Elements[1:]...)
	}
}

func (q *Queue) IsEmpty() bool {
	return len(q.Elements) == 0
}

func (q *Queue) GetLength() int {
	return len(q.Elements)
}
