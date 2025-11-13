package router

import (
	"container/heap"
	"math"

	"github.com/shopspring/decimal"
)

const MaxHops = 3

type Pathfinder struct {
	pools  []AMMPool
	offers []Offer
}

func NewPathfinder(pools []AMMPool, offers []Offer) *Pathfinder {
	return &Pathfinder{
		pools:  pools,
		offers: offers,
	}
}

func (pf *Pathfinder) FindBestRoute(in, out Asset, amount decimal.Decimal) (*Route, error) {
	graph := pf.buildGraph()
	
	path, cost := pf.dijkstra(graph, in.String(), out.String())
	if path == nil {
		return nil, ErrNoRoute
	}

	if len(path) > MaxHops+1 {
		return nil, ErrNoRoute
	}

	route := pf.buildRoute(path, amount)
	if route == nil {
		return nil, ErrInsufficientLiquidity
	}

	_ = cost
	return route, nil
}

type edge struct {
	to     string
	weight decimal.Decimal
	pool   *AMMPool
	offer  *Offer
}

type node struct {
	asset    string
	cost     decimal.Decimal
	index    int
}

type priorityQueue []*node

func (pq priorityQueue) Len() int           { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool { return pq[i].cost.LessThan(pq[j].cost) }
func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*node)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

func (pf *Pathfinder) buildGraph() map[string][]edge {
	graph := make(map[string][]edge)

	for i := range pf.pools {
		pool := &pf.pools[i]
		asset1 := Asset{Currency: pool.Asset1.Currency, Issuer: pool.Asset1.Issuer}.String()
		asset2 := Asset{Currency: pool.Asset2.Currency, Issuer: pool.Asset2.Issuer}.String()

		feeMultiplier := decimal.NewFromInt(1).Sub(
			decimal.NewFromInt(int64(pool.TradingFeeBps)).Div(decimal.NewFromInt(10000)),
		)

		graph[asset1] = append(graph[asset1], edge{
			to:     asset2,
			weight: feeMultiplier,
			pool:   pool,
		})
		graph[asset2] = append(graph[asset2], edge{
			to:     asset1,
			weight: feeMultiplier,
			pool:   pool,
		})
	}

	for i := range pf.offers {
		offer := &pf.offers[i]
		from := offer.TakerPays.String()
		to := offer.TakerGets.String()

		graph[from] = append(graph[from], edge{
			to:     to,
			weight: offer.Quality,
			offer:  offer,
		})
	}

	return graph
}

func (pf *Pathfinder) dijkstra(graph map[string][]edge, start, end string) ([]string, decimal.Decimal) {
	dist := make(map[string]decimal.Decimal)
	prev := make(map[string]string)
	visited := make(map[string]bool)

	for k := range graph {
		dist[k] = decimal.NewFromFloat(math.MaxFloat64)
	}
	dist[start] = decimal.Zero

	pq := make(priorityQueue, 0)
	heap.Init(&pq)
	heap.Push(&pq, &node{asset: start, cost: decimal.Zero})

	for pq.Len() > 0 {
		current := heap.Pop(&pq).(*node)

		if visited[current.asset] {
			continue
		}
		visited[current.asset] = true

		if current.asset == end {
			break
		}

		for _, e := range graph[current.asset] {
			if visited[e.to] {
				continue
			}

			newCost := current.cost.Add(decimal.NewFromInt(1).Sub(e.weight))
			if newCost.LessThan(dist[e.to]) {
				dist[e.to] = newCost
				prev[e.to] = current.asset
				heap.Push(&pq, &node{asset: e.to, cost: newCost})
			}
		}
	}

	if _, ok := prev[end]; !ok && start != end {
		return nil, decimal.Zero
	}

	path := []string{}
	for at := end; at != ""; at = prev[at] {
		path = append([]string{at}, path...)
		if at == start {
			break
		}
	}

	return path, dist[end]
}

func (pf *Pathfinder) buildRoute(path []string, amount decimal.Decimal) *Route {
	if len(path) < 2 {
		return nil
	}

	route := &Route{
		Hops: make([]Hop, 0, len(path)-1),
	}

	currentAmount := amount

	for i := 0; i < len(path)-1; i++ {
		hop := pf.findHop(path[i], path[i+1], currentAmount)
		if hop == nil {
			return nil
		}

		route.Hops = append(route.Hops, *hop)
		currentAmount = hop.AmountOut
	}

	return route
}

func (pf *Pathfinder) findHop(from, to string, amountIn decimal.Decimal) *Hop {
	for i := range pf.pools {
		pool := &pf.pools[i]
		asset1 := pool.Asset1.String()
		asset2 := pool.Asset2.String()

		if asset1 == from && asset2 == to {
			amountOut := pf.calculateAMMOutput(pool, amountIn, true)
			return &Hop{
				Type:      "amm",
				In:        pool.Asset1,
				Out:       pool.Asset2,
				AmountIn:  amountIn,
				AmountOut: amountOut,
			}
		}

		if asset2 == from && asset1 == to {
			amountOut := pf.calculateAMMOutput(pool, amountIn, false)
			return &Hop{
				Type:      "amm",
				In:        pool.Asset2,
				Out:       pool.Asset1,
				AmountIn:  amountIn,
				AmountOut: amountOut,
			}
		}
	}

	for i := range pf.offers {
		offer := &pf.offers[i]
		if offer.TakerPays.String() == from && offer.TakerGets.String() == to {
			amountOut := amountIn.Mul(offer.Quality)
			return &Hop{
				Type:      "orderbook",
				In:        offer.TakerPays,
				Out:       offer.TakerGets,
				AmountIn:  amountIn,
				AmountOut: amountOut,
			}
		}
	}

	return nil
}

func (pf *Pathfinder) calculateAMMOutput(pool *AMMPool, amountIn decimal.Decimal, asset1ToAsset2 bool) decimal.Decimal {
	var reserveIn, reserveOut decimal.Decimal

	if asset1ToAsset2 {
		reserveIn = pool.Asset1Reserve
		reserveOut = pool.Asset2Reserve
	} else {
		reserveIn = pool.Asset2Reserve
		reserveOut = pool.Asset1Reserve
	}

	feeMultiplier := decimal.NewFromInt(1).Sub(
		decimal.NewFromInt(int64(pool.TradingFeeBps)).Div(decimal.NewFromInt(10000)),
	)

	amountInAfterFee := amountIn.Mul(feeMultiplier)
	numerator := amountInAfterFee.Mul(reserveOut)
	denominator := reserveIn.Add(amountInAfterFee)

	return numerator.Div(denominator)
}
