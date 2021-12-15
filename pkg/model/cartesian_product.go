package model

// CartesianProduct takes map of lists and returns list of unique tuples
func cartesianProduct(mapOfLists map[string][]interface{}) []map[string]interface{} {
	listNames := make([]string, 0)
	lists := make([][]interface{}, 0)
	for k, v := range mapOfLists {
		listNames = append(listNames, k)
		lists = append(lists, v)
	}

	listCart := cartN(lists...)

	rtn := make([]map[string]interface{}, 0)
	for _, list := range listCart {
		vMap := make(map[string]interface{})
		for i, v := range list {
			vMap[listNames[i]] = v
		}
		rtn = append(rtn, vMap)
	}
	return rtn
}

func cartN(a ...[]interface{}) [][]interface{} {
	c := 1
	for _, a := range a {
		c *= len(a)
	}
	if c == 0 || len(a) == 0 {
		return nil
	}
	p := make([][]interface{}, c)
	b := make([]interface{}, c*len(a))
	n := make([]int, len(a))
	s := 0
	for i := range p {
		e := s + len(a)
		pi := b[s:e]
		p[i] = pi
		s = e
		for j, n := range n {
			pi[j] = a[j][n]
		}
		for j := len(n) - 1; j >= 0; j-- {
			n[j]++
			if n[j] < len(a[j]) {
				break
			}
			n[j] = 0
		}
	}
	return p
}
