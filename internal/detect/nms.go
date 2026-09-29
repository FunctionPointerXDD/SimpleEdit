package detect

import "sort"

// 점수 내림차순으로 보며, 이미 남긴 박스와 IoU가 기준을 넘으면 버린다.
func NMS(boxes []Box, iouThreshold float32) []Box {
	sorted := append([]Box(nil), boxes...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Score > sorted[j].Score })
	keep := make([]Box, 0, len(sorted))
	for _, b := range sorted {
		ok := true
		for _, k := range keep {
			if IoU(b, k) > iouThreshold {
				ok = false
				break
			}
		}
		if ok {
			keep = append(keep, b)
		}
	}
	return keep
}

// 교집합 넓이 / 합집합 넓이. 겹치지 않으면 0
func IoU(a, b Box) float32 {
	x0, y0 := max(a.X, b.X), max(a.Y, b.Y)
	x1, y1 := min(a.X+a.W, b.X+b.W), min(a.Y+a.H, b.Y+b.H)
	if x1 <= x0 || y1 <= y0 {
		return 0
	}
	inter := (x1 - x0) * (y1 - y0)
	return inter / (a.W*a.H + b.W*b.H - inter)
}
