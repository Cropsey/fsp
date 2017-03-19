package fsp

import (
	"fmt"
	"math"
	"math/rand"
	"time"
	"os"
	//"sort"
	//"github.com/pkg/profile"
)

// Freaky engine finding pseudo-ant paths
type AntEngine struct {
	graph Graph
	seed  int
}

var feromones []float32
type ant struct {
	day Day
	city City
	total Money
	visited []City
	fis []FlightIndex
}

var ANTS = 20
var ants []ant

var antCurrentBest = Money(math.MaxInt32)

func (e AntEngine) Name() string {
	return fmt.Sprintf("%s(%d)", "AntEngine", e.seed)
}

func (e AntEngine) Solve(comm comm, p Problem) {
	//defer profile.Start(/*profile.MemProfile*/).Stop()
	rand.Seed(int64(e.seed) + time.Now().UTC().UnixNano())
	feromones = make([]float32, len(p.flights))
	antInit(ANTS, p.n)
	antSolver(p, e.graph, comm)
	//comm.done()
}

func antInit(ant_n, problem_n int) {
	ants = make([]ant, ant_n, ant_n)
	for ai := range ants {
		ants[ai].visited = make([]City, 0, problem_n)
		ants[ai].fis = make([]FlightIndex, 0, problem_n)
	}
}

func antSolver(problem Problem, graph Graph, comm comm) {
	//solution := make([]Flight, 0, graph.size)
	antsFinished := 0
	for {
		minTotal := ants[0].total
		minIndex := 0
		for ai := range ants {
			if ants[ai].total < minTotal {
				minTotal = ants[ai].total
				minIndex = ai
			}
		}
		ai := minIndex // the chosen one
		//printInfo("The chosen one", ai, ants[ai])
		fi, r := antFlight(problem, graph, ants[ai].visited, ants[ai].day, ants[ai].city)
		if !r {
			//printInfo("ant to die", ai, ants[ai].visited, "day", d, "city", ants[ai].city)
			die(ai)
			continue
		}
		//printInfo("FI:", fi)
		flight := problem.flights[fi]
		ants[ai].total += flight.Cost
		ants[ai].day++
		ants[ai].city = flight.To
		if ants[ai].city == 0 { // ant has completed the route
			ants[ai].day = 0
			// place the feromones
			for _, fi := range ants[ai].fis {
				feromones[fi] += 1.0
			}
			ants[ai].visited = ants[ai].visited[:0]
			ants[ai].fis = ants[ai].fis[:0]
			antsFinished++
		} else {
			ants[ai].visited = append(ants[ai].visited, ants[ai].city)
			ants[ai].fis = append(ants[ai].fis, fi)
		}
		if antsFinished > ANTS * 10 {
			//printInfo("ants finished")
			antsFinished = 0
			mf := float32(0.0)
			for fi := range feromones {
				feromones[fi] *= 0.85
				if feromones[fi] > mf { mf = feromones[fi] }
			}
			printInfo("Max feromone:", mf)
			//printInfo("Feromones:", feromones)
			printInfo("ants following")
			followAnts(problem, graph, comm)
			printInfo("ants followed")
		}
	}
}

func followAnts(problem Problem, graph Graph, comm comm) {
	solution := make([]Flight, 0, graph.size)
	var price Money
	var city City
	for {
		solution = solution[:0]
		visited := make([]City, 0, MAX_CITIES)
		city = City(0)
		price = Money(0)
		for d := 0; d < graph.size; d++ {
			fi, r := antFlight(problem, graph, visited, Day(d), city)
			if !r {
				return
			}
			price += problem.flights[fi].Cost
			if price >= randomCurrentBest {
				break
			}
			city = problem.flights[fi].To
			visited = append(visited, city)
			solution = append(solution, problem.flights[fi])
		}
		if len(solution) == graph.size && price < antCurrentBest {
			antCurrentBest = price
			comm.sendSolution(NewSolution(solution))
			printInfo("ant solution sent, price", price)
			return
		}
	}
}

func die(ai int) {
	//printInfo("ant", ai, "dying")
	ants[ai].day = 0
	ants[ai].city = 0
	ants[ai].visited = ants[ai].visited[:0]
	ants[ai].fis = ants[ai].fis[:0]
	// keep current total cost for now; maybe add maximum flight cost or assign current worst running ant total
}

// ants don't fly

func antWeight(problem Problem, fi FlightIndex) float32 {
	var FEROM_C float64 = 1.0
	var PRICE_C float64 = 1.0
	var AVG_FLIGHT_COST float64 = 500.0
	// feromones influence
	f := math.Pow(float64(feromones[fi]+1.0)/float64(ANTS), FEROM_C)
	// price influence
	p := math.Pow(AVG_FLIGHT_COST/float64(problem.flights[fi].Cost), PRICE_C)
	var result float32 = float32(f*p)
//printInfo("f/p:", f, "/", p, "feromones", feromones[fi], "cost", problem.flights[fi].Cost)
fmt.Fprintf(os.Stderr, "f/p: %.4f * %.2f = %.2f, (feromones %.2f, cost %v)\n", f, p, result, feromones[fi], problem.flights[fi].Cost)
	return result
}

// choose the flight ant will take
func antFlight(problem Problem, graph Graph, visited []City, day Day, city City) (FlightIndex, bool) {
	// first, find all possible flights and construct random distribution
	possible_flights := make([]FlightIndex, 0, MAX_CITIES)
	thres := make([]float32, 0, MAX_CITIES+1) // array of thresholds
	thres = append(thres, 0.0) // easier logic later if we always start with 0.0
	var fsum float32 = 0.0
	for _, fi := range graph.antsGraph[city][day] {
		if contains(visited, problem.flights[fi].To) {
			continue
		}
		possible_flights = append(possible_flights, fi)
		// compute weight of the flight
		// TODO scale according to average flight price
		fsum += antWeight(problem, fi)
		//fsum += float32(graph.size) + (10.0*float32(graph.size)/float32(ANTS))*feromones[fi]
		thres = append(thres, fsum)
	}
	flightCnt := len(possible_flights)

	// second, return that ant is stuck if no flight possible
	if flightCnt == 0 {
		return 0, false
	}

	// third, choose flight randomly based on the distribution
	r := rand.Float32() * fsum
	result := flightCnt - 1
	for i, f := range thres {
		if r < f {
			result = i-1
			break
		}
	}
	return possible_flights[result], true
}
