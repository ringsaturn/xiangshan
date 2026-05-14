package geom

import "math"

const yStripesMin = 32

type yStripe struct {
	start, count int
}

// yStripesIndex partitions the segments of one ring into horizontal stripes.
// A PIP lookup for latitude y only needs to examine the segments stored in
// the single stripe that contains y.
type yStripesIndex struct {
	minY    float64
	height  float64
	stripes []yStripe
	indexes []int
	yRanges [][2]float64
}

func calcStripeCount(r Ring) int {
	area, perim := ringAreaAndPerimeter(r)
	score := 0.0
	if perim > 0 {
		score = (area * math.Pi * 4) / (perim * perim)
	}
	n := int(math.Floor(float64(len(r)) * score))
	if n < yStripesMin {
		return yStripesMin
	}
	return n
}

func segStripeRange(segMinY, segMaxY, minY, height float64, count int) (lo, hi int) {
	if count <= 1 || height == 0 {
		return 0, 0
	}
	last := count - 1
	lo = clampStripe(int(math.Floor((segMinY-minY)/height*float64(count))), last)
	hi = clampStripe(int(math.Floor((segMaxY-minY)/height*float64(count))), last)
	return
}

func clampStripe(i, last int) int {
	if i < 0 {
		return 0
	}
	if i > last {
		return last
	}
	return i
}

func pointStripe(y, minY, height float64, count int) int {
	return clampStripe(int(math.Floor((y-minY)/height*float64(count))), count-1)
}

func buildYStripes(r Ring) *yStripesIndex {
	n := len(r)
	if n < 2 {
		return nil
	}

	yRanges := make([][2]float64, n)
	minY := r[0].Y
	maxY := r[0].Y
	for i := range n {
		j := (i + 1) % n
		ay, by := r[i].Y, r[j].Y
		if ay <= by {
			yRanges[i] = [2]float64{ay, by}
		} else {
			yRanges[i] = [2]float64{by, ay}
		}
		if yRanges[i][0] < minY {
			minY = yRanges[i][0]
		}
		if yRanges[i][1] > maxY {
			maxY = yRanges[i][1]
		}
	}

	height := maxY - minY
	if height == 0 {
		return nil
	}

	stripeCount := calcStripeCount(r)
	stripes := make([]yStripe, stripeCount)

	for i := range n {
		lo, hi := segStripeRange(yRanges[i][0], yRanges[i][1], minY, height, stripeCount)
		for s := lo; s <= hi; s++ {
			stripes[s].count++
		}
	}

	total := 0
	starts := make([]int, stripeCount)
	for s := range stripes {
		starts[s] = total
		stripes[s].start = total
		total += stripes[s].count
		stripes[s].count = 0
	}

	indexes := make([]int, total)
	for i := range n {
		lo, hi := segStripeRange(yRanges[i][0], yRanges[i][1], minY, height, stripeCount)
		for s := lo; s <= hi; s++ {
			pos := starts[s] + stripes[s].count
			indexes[pos] = i
			stripes[s].count++
		}
	}

	return &yStripesIndex{
		minY:    minY,
		height:  height,
		stripes: stripes,
		indexes: indexes,
		yRanges: yRanges,
	}
}

func (idx *yStripesIndex) forEachCandidate(y float64, fn func(int) bool) {
	if y < idx.minY || y > idx.minY+idx.height {
		return
	}
	s := pointStripe(y, idx.minY, idx.height, len(idx.stripes))
	stripe := idx.stripes[s]
	for k := stripe.start; k < stripe.start+stripe.count; k++ {
		seg := idx.indexes[k]
		if y >= idx.yRanges[seg][0] && y <= idx.yRanges[seg][1] {
			if !fn(seg) {
				return
			}
		}
	}
}
