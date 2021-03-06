package fsp

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	//"github.com/pkg/profile"
)

// Depth + Cheapest First Search engine
// a variant of greedy DFS using cheapest next flight first with heuristics based on average price for same flights on different days
type Dcfs struct {
	graph Graph
	skip  int
}

var DcfsResultsCounter uint32
var DcfsBranchCounter []uint32
var dcfsCurrentBest = Money(math.MaxInt32)

// engine parms
var pMaxBranches = MAX_CITIES
var pDiscountWeight = float32(0.6)
var pNextAvgWeight = float32(-0.2)
var pMinDiscount = float32(-0.5)
var pDiscountThreshold = Money(0) // set as avg flight price in Solve()

func (e Dcfs) Name() string {
	return fmt.Sprintf("%s(%d)", "Dcfs", e.skip)
}

func dcfsLoadEnvParams() {
	var env string
	env = os.Getenv("DCFS_MAX_BRANCHES")
	if len(env) > 0 {
		pMaxBranches, _ = strconv.Atoi(env)
	}
	env = os.Getenv("DCFS_DISC_W")
	if len(env) > 0 {
		tmp, _ := strconv.ParseFloat(env, 32)
		pDiscountWeight = float32(tmp)
	}
	env = os.Getenv("DCFS_NEXT_AVG_W")
	if len(env) > 0 {
		tmp, _ := strconv.ParseFloat(env, 32)
		pNextAvgWeight = float32(tmp)
	}
	env = os.Getenv("DCFS_MIN_DISC")
	if len(env) > 0 {
		tmp, _ := strconv.ParseFloat(env, 32)
		pMinDiscount = float32(tmp)
	}
	env = os.Getenv("DCFS_DISC_THRESH")
	if len(env) > 0 {
		tmp, _ := strconv.Atoi(env)
		pDiscountThreshold = Money(tmp)
	}
}

func (e Dcfs) Solve(comm comm, p Problem) {
	//defer profile.Start(/*profile.MemProfile*/).Stop()
	DcfsBranchCounter = make([]uint32, e.graph.size+1)
	pDiscountThreshold = Money(p.FlightStats().AvgPrice)
	if p.n > 20 {
		pMaxBranches = 2
	} else {
		pMaxBranches = p.n / 2
	}
	if e.skip > 0 && p.n < 20 {
		return
	}
	dcfsLoadEnvParams()
	dcfsSolver(e.graph, p.stats, comm, e.skip)
	//comm.done()
}

type EvaluatedFlight struct {
	flight Flight
	value  float32
}

type byValue []EvaluatedFlight

func (f byValue) Len() int {
	return len(f)
}

func (f byValue) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

func (f byValue) Less(i, j int) bool {
	return f[i].value < f[j].value
}

func dcfsInsertSortedFlight(slice []EvaluatedFlight, node EvaluatedFlight) []EvaluatedFlight {
	l := len(slice)
	if l == 0 {
		return []EvaluatedFlight{node}
	}
	i := sort.Search(l, func(i int) bool { return slice[i].value > node.value })
	//fmt.Println(i)
	if i == 0 {
		return append([]EvaluatedFlight{node}, slice...)
	}
	if i == -1 {
		return append(slice[0:l], node)
	}
	//tail := append([]EvaluatedFlight{node}, slice[i:]...)
	return append(slice[0:i], append([]EvaluatedFlight{node}, slice[i:l]...)...)
}

func dcfsInsertVisited(slice []City, c City) []City {
	l := len(slice)
	if l == 0 {
		return []City{c}
	}
	i := sort.Search(l, func(i int) bool { return slice[i] > c })
	if i == 0 {
		return append([]City{c}, slice...)
	}
	if i == -1 {
		return append(slice[0:l], c)
	}
	//tail := append([]EvaluatedFlight{node}, slice[i:]...)
	return append(slice[0:i], append([]City{c}, slice[i:l]...)...)
}

func dcfsVisited(slice []City, c City) bool {
	l := len(slice)
	if l < 1 {
		return false
	}
	p := sort.Search(l, func(i int) bool { return slice[i] >= c })
	if p < l && slice[p] == c {
		return true
	}
	return false
}

func dcfsSolver(graph Graph, stats FlightStatistics, comm comm, skip int) /*[]Flight*/ {

	printInfo("starting dcfs solver", skip)
	visited := make([]City, 0, MAX_CITIES)
	solution := make([]Flight, 0, graph.size)
	home := City(0)
	day := Day(0)
	price := Money(0)
	dcfsIterate(solution, day, home, visited, graph, stats, price, comm, skip)
}

func dcfsIterate(partial []Flight, day Day, current City,
	visited []City, graph Graph, stats FlightStatistics, price Money, comm comm, skip int) {

	DcfsBranchCounter[day] += 1
	if price >= dcfsCurrentBest {
		// we have already got worse than best result, give it up, bro
		DcfsResultsCounter++
		return
	}
	if int(day) == graph.size {
		DcfsResultsCounter++
		if price < dcfsCurrentBest {
			//dcfsCurrentBest = price
			dcfsCurrentBest = comm.sendSolution(NewSolution(partial))
		}
		return
	}
	//fmt.Fprintln(os.Stderr, "I am at", current, "day is", day)
	var current_deal float32
	//var current_deal int32
	possible_flights := make([]EvaluatedFlight, 0, MAX_CITIES)
	for _, f := range graph.fromDaySortedCost[current][day] {
		//printInfo(f)
		if contains(visited, f.To) {
			//if dcfsVisited(visited, f.To) {
			continue
		}
		s := stats.ByDest[current][f.To]
		discount := s.AvgPrice - float32(f.Cost)
		discount_rate := discount / float32(f.Cost)
		var s2 FlightStats
		if day < Day(len(stats.ByDay[f.To])-1) {
			s2 = stats.ByDay[f.To][day+1]
		}
		//if discount_rate < -0.3 {
		if f.Cost > pDiscountThreshold && discount_rate < pMinDiscount {
			// no discount, no deal, bro
			continue
		}
		//current_deal = float32(f.Cost) - s.AvgPrice * discount // - NO NO NO
		//current_deal = float32(f.Cost) * s.AvgPrice - s.AvgPrice * discount // (200, 300) = 39639, 51790
		//current_deal = float32(f.Cost) - 0.3 * discount // (200, 300) = 40722, 51625
		//current_deal = float32(f.Cost) - 0.6 * discount // (200, 300) = 40543, 48493
		//current_deal = float32(f.Cost) - 0.9 * discount // (200, 300) = 40447, 50580, total: 189785
		//current_deal = float32(f.Cost) // (200, 300) = NO, 48288, total: 189716
		//current_deal = s.AvgPrice // (200, 300) = NO, NO, total: 193574
		//current_deal = float32(f.Cost) * s.AvgPrice // (200, 300) = NO, NO total: 192248
		//current_deal = float32(f.Cost) * s.AvgPrice + stats[f.To][current].AvgPrice * float32(f.Cost) // (200, 300) = NO, NO total: 192788
		//current_deal = float32(f.Cost) - discount_rate * discount // (200, 300) = NO, 50481 total: 190949
		//current_deal = -discount // no result total 194138
		//current_deal = float32(f.Cost) - 0.6 * discount // (200, 300) = No, 48590, total: 187078 (disc rate < 0.3)
		//current_deal = float32(f.Cost) - 0.6*discount // (200, 300) = 40505, 48493, total: 187010 (disc rate < 0.25, >650)
		current_deal = float32(f.Cost) - pDiscountWeight*discount + pNextAvgWeight*s2.AvgPrice // (200, 300) = 40505, 48493, total: 187010 (disc rate < 0.25, >650)

		//possible_flights = append(possible_flights, EvaluatedFlight{f, current_deal})
		possible_flights = dcfsInsertSortedFlight(possible_flights, EvaluatedFlight{*f, current_deal})
	}
	//sort.Sort(byValue(possible_flights))
	if len(possible_flights) == 0 {
		return
	}
	if len(possible_flights) > pMaxBranches && day > 0 {
		possible_flights = possible_flights[:pMaxBranches]
	}
	last_value := possible_flights[0].value
	for i, f := range possible_flights {
		if f.value-last_value > 30 {
			return
		}
		if day == 0 && skip > i {
			skip--
			continue
		}
		dcfsIterate(append(partial, f.flight),
			day+1,
			f.flight.To,
			append(visited, f.flight.To),
			//dcfsInsertVisited(visited, f.flight.To),
			graph, stats,
			price+f.flight.Cost,
			comm, skip)
		last_value = f.value

	}
	return //[]Flight{}
}
