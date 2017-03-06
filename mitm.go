package fsp

import (
	//"fmt"
	"github.com/emef/bitfield"
	"math"
)

type Mitm struct{} // meet in the middle

func (m Mitm) Name() string {
	return "MeetInTheMiddle"
}

func (m Mitm) Solve(comm comm, problem Problem) {
	if problem.n < 2 {
		comm.sendSolution(Solution{})
		return
	}
	// processing Problem into two trees
	there, back := makeTwoTrees(problem)
	var mps meetPlaces = make(map[City]meetPlace)

	// we
	left := make(chan halfRoute)
	right := make(chan halfRoute)

	// run, Forrest!
	go startHalfDFS(left, problem, &there)
	go startHalfDFS(right, problem, &back)

	var found *[]City = nil
	var hr halfRoute
	var ok bool
	var bestCost Money = Money(math.MaxInt32)
	var solution Solution
	for {
		select {
		case hr, ok = <-left:
			if !ok {
				left = nil
			} else {
				found = mps.add(true, &hr)
			}
		case hr, ok = <-right:
			if !ok {
				right = nil
			} else {
				found = mps.add(false, &hr)
			}
		}
		if found != nil {
			solution = problem.route2solution(*found)
			if solution.totalCost < bestCost {
				bestCost = solution.totalCost
				comm.sendSolution(solution)
			}
			found = nil
		}
		if left == nil && right == nil {
			comm.done()
			break
		}
	}
}

type citySet struct {
	n    int
	data bitfield.BitField
}

func csInit(n int) (cs citySet) {
	cs.n = n
	cs.data = bitfield.New(n)
	return
}
func (cs citySet) add(c City) citySet {
	cs.data.Set(uint32(c))
	return cs
}
func (cs citySet) test(c City) bool {
	return cs.data.Test(uint32(c))
}

//TODO this is terrible name, make something better
func (cs citySet) allVisited(other citySet) bool {
	var bi uint32
	for i := 0; i < other.n; i++ {
		bi = uint32(i)
		ob := other.data.Test(bi)
		cb := cs.data.Test(bi)
		if !(cb || ob) {
			return false
		}
	}
	return true
}
func (cs citySet) full() bool { //naive, could be more efficient
	for i := 0; i < cs.n; i++ {
		if !cs.data.Test(uint32(i)) {
			return false
		}
	}
	return true
}
func (cs citySet) String() string {
	res := make([]byte, cs.n)
	for i := 0; i < cs.n; i++ {
		if cs.data.Test(uint32(i)) {
			res[i] = '1'
		} else {
			res[i] = '0'
		}
	}
	return string(res)
}

//IDEA
// could be probably optimized to
// map[Day]map[City][int]
// where those ints are indexes to Problem.Flights sorted by cost
type flightTree map[Day]map[City][]flightTo

type halfRoute struct {
	visited citySet
	route   []City
	cost Money
}
type meetPlaces map[City]meetPlace

type meetPlace struct {
	left, right *[]halfRoute
}

// returns route if full route can be constructed, otherwise nil
func (mps meetPlaces) add(left bool, hr *halfRoute) *[]City {
	city := (*hr).route[len((*hr).route)-1]
	mp, present := mps[city]
	if !present {
		l := []halfRoute{}
		r := []halfRoute{}
		if left {
			l = append(l, *hr)
		} else {
			r = append(r, *hr)
		}
		mps[city] = meetPlace{&l, &r}
		mp = mps[city]
	}
	hrsCurrent := mp.left
	hrsOther := mp.right
	if !left {
		hrsCurrent = mp.right
		hrsOther = mp.left
	}
	bestCost := Money(math.MaxInt32)
	// TODO consider cost
	var found *halfRoute = nil
	for _, v := range *hrsOther {
		if v.visited.allVisited(hr.visited) {
			if v.cost < bestCost {
				found = &v
				bestCost = v.cost
			}
		}
	}
	hrsNew := append(*hrsCurrent, *hr)
	hrsCurrent = &hrsNew
	if found != nil {
		result := make([]City, 0, (*hr).visited.n)
		if left {
			result = append(result, hr.route...)
			for i := len((*found).route) - 2; i >= 1; i-- {
				result = append(result, (*found).route[i])
			}
		} else {
			result = append(result, (*found).route...)
			for i := len((*hr).route) - 2; i >= 1; i-- {
				result = append(result, ((*hr).route)[i])
			}
		}
		return &result
	}
	return nil
}

// wrapper around halfDFS
func startHalfDFS(output chan halfRoute, problem Problem, ft *flightTree) {
	defer close(output)

	visited := csInit(problem.n)
	visited.add(problem.start)
	halfDFS(output, []City{problem.start}, visited, 0, Day(len(*ft)), 0, ft)
}

func halfDFS(output chan halfRoute, partial []City, visited citySet, day, endDay Day, cost Money, ft *flightTree) {
	if day == endDay {
		// we have reached the meeting day
		output <- halfRoute{visited, partial, cost}
		return
	}
	lastVisited := partial[len(partial)-1]
	//TODO not looking at cost at all
	for _, fl := range (*ft)[day][lastVisited] {
		city := fl.to
		if !visited.test(city) {
			halfDFS(output, append(partial, city),
				visited.add(city),
				day+1, endDay, cost+fl.cost, ft)
		}
	}
	return
}

type flightTo struct {
	to City
	cost Money
}

func addFlight(ft *flightTree, day Day, from, to City, cost Money, n int) {
	if (*ft) == nil {
		(*ft) = make(map[Day]map[City][]flightTo)
	}
	if (*ft)[day] == nil {
		(*ft)[day] = make(map[City][]flightTo)
	}
	if (*ft)[day][from] == nil {
		//(*ft)[day][from] = make(map[City]Money)
		(*ft)[day][from] = make([]flightTo, 0, n)
	}
	insertIndex := 0
	for _, v := range (*ft)[day][from] {
		if cost < v.cost {
			break
		}
		insertIndex++
	}
	(*ft)[day][from] = append((*ft)[day][from][:insertIndex],
				append([]flightTo{flightTo{to, cost}},
					(*ft)[day][from][insertIndex:]...)...)
	//(*ft)[day][from][to] = cost
}

func makeTwoTrees(problem Problem) (there, back flightTree) {
	// get the number of days
	var days Day = Day(problem.n)
	meetDay := days / 2
	for _, f := range problem.flights {
		if f.Day < meetDay {
			addFlight(&there, f.Day, f.From, f.To, f.Cost, problem.n)
		} else {
			addFlight(&back, days-1-f.Day, f.To, f.From, f.Cost, problem.n)
		}
	}
	return
}
