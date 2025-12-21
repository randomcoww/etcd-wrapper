package util

func HasMatchingElement[T comparable](s1, s2 []T) bool {
	elementsInS1 := make(map[T]struct{})
	for _, v := range s1 {
		elementsInS1[v] = struct{}{}
	}
	for _, v := range s2 {
		if _, ok := elementsInS1[v]; ok {
			return true
		}
	}
	return false
}
