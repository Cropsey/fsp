package fsp

import (
	"math"
	"sort"
)

type Bottleneck struct {
	graph       Graph
	currentBest Money
}

func NewBottleneck(g Graph) Bottleneck {
	return Bottleneck{
		graph,
		Money(math.MaxInt32),
	}

}

func (d Bottleneck) Name() string {
	return "Bottleneck"
}

type byCost2 []Flight

func (f byCost2) Len() int {
	return len(f)
}
func (f byCost2) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}
func (f byCost2) Less(i, j int) bool {
	return f[i].Cost < f[j].Cost
}

func (d Bottleneck) Solve(comm comm, problem Problem) {
	flights := make([]Flight, 0, problem.n)
	visited := make(map[City]bool)
	partial := partial{visited, flights, problem.n, 0}
	for _, b := range d.findBottlenecks(problem) {
		sort.Sort(byCost2(b))
		for _, f := range b {
			printInfo("Bottleneck starting with", f)
			partial.fly(f)
			d.dfs(comm, &partial)
			partial.backtrack()
		}
	}
	printInfo("Bottleneck finished")
}

type byCount [][]Flight

func (f byCount) Len() int {
	return len(f)
}
func (f byCount) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}
func (f byCount) Less(i, j int) bool {
	return len(f[i]) < len(f[j])
}

type btnStat struct {
	from    [][]Flight
	to      [][]Flight
	noBFrom []bool
	noBTo   []bool
	cutoff  int
}

func initB(n int) btnStat {
	b := btnStat{}
	b.from = make([][]Flight, n)
	b.to = make([][]Flight, n)
	for i := range b.from {
		b.from[i] = make([]Flight, 0, n)
		b.to[i] = make([]Flight, 0, n)
	}
	b.noBFrom = make([]bool, n)
	b.noBTo = make([]bool, n)
	b.cutoff = n / 4
	return b
}

func (b *btnStat) add(f Flight) {
	if !b.noBFrom[f.From] {
		b.from[f.From] = append(b.from[f.From], f)
		if len(b.from[f.From]) > b.cutoff {
			b.from[f.From] = nil
			b.noBFrom[f.From] = true
		}
	}
	if !b.noBTo[f.To] {
		b.to[f.To] = append(b.to[f.To], f)
		if len(b.to[f.To]) > b.cutoff {
			b.to[f.To] = nil
			b.noBTo[f.To] = true
		}
	}
}

func (b btnStat) get() [][]Flight {
	all := make([][]Flight, 0, len(b.from)+len(b.to))
	for _, f := range b.from {
		if f != nil {
			all = append(all, f)
		}
	}
	for _, f := range b.to {
		if f != nil {
			all = append(all, f)
		}
	}
	sort.Sort(byCount(all))
	return all
}

func (b *Bottleneck) findBottlenecks(p Problem) [][]Flight {
	bs := initB(p.n)
	for _, f := range p.flights {
		if f.From == 0 || f.To == 0 {
			continue
		}
		bs.add(f)
	}
	return bs.get()
}

func (b *Bottleneck) dfs(comm comm, partial *partial) {
	if partial.cost > b.currentBest {
		return
	}
	if partial.roundtrip() {
		sf := make([]Flight, partial.n)
		copy(sf, partial.flights)
		sort.Sort(ByDay(sf))
		b.currentBest = comm.sendSolution(NewSolution(sf))
	}

	lf := partial.lastFlight()
	if partial.hasVisited(lf.To) {
		return
	}

	dst := b.graph.fromDaySortedCost[lf.To][int(lf.Day+1)%b.graph.size]
	for _, f := range dst {
		partial.fly(*f)
		b.dfs(comm, partial)
		partial.backtrack()
	}
}