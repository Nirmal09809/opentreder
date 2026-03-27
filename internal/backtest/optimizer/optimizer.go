package optimizer

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"sync"
	"time"

	"github.com/opentreder/opentreder/pkg/logger"
)

type Optimizer struct {
	config    *Config
	strategy  StrategyBacktester
	population []*Individual
	mu        sync.RWMutex
}

type Config struct {
	PopulationSize int
	Generations    int
	MutationRate   float64
	CrossoverRate float64
	EliteCount    int
	Objective     string
	Workers       int
}

type Individual struct {
	ID        string
	Genes     map[string]interface{}
	Fitness   float64
	Metrics   FitnessMetrics
	CreatedAt time.Time
}

type FitnessMetrics struct {
	TotalReturn     float64
	SharpeRatio     float64
	MaxDrawdown     float64
	WinRate         float64
	ProfitFactor    float64
	TotalTrades     int
	AvgTrade        float64
	SortinoRatio    float64
	CalmarRatio     float64
}

type StrategyBacktester interface {
	Backtest(params map[string]interface{}) (*BacktestResult, error)
}

type BacktestResult struct {
	TotalReturn     float64
	SharpeRatio     float64
	MaxDrawdown     float64
	WinRate         float64
	ProfitFactor    float64
	TotalTrades     int
	AvgTrade        float64
	SortinoRatio    float64
	CalmarRatio     float64
	EquityCurve     []float64
}

type OptimizationResult struct {
	BestIndividual *Individual
	Population     []*Individual
	GenerationsRun int
	TimeTaken     time.Duration
	Converged     bool
}

type ParameterSpace struct {
	Name     string
	Type     string
	Min      float64
	Max      float64
	Step     float64
	Values   []interface{}
}

func NewOptimizer(cfg *Config, strategy StrategyBacktester) *Optimizer {
	if cfg.PopulationSize == 0 {
		cfg.PopulationSize = 50
	}
	if cfg.Generations == 0 {
		cfg.Generations = 100
	}
	if cfg.MutationRate == 0 {
		cfg.MutationRate = 0.1
	}
	if cfg.CrossoverRate == 0 {
		cfg.CrossoverRate = 0.7
	}
	if cfg.EliteCount == 0 {
		cfg.EliteCount = 5
	}

	return &Optimizer{
		config:    cfg,
		strategy:  strategy,
		population: make([]*Individual, 0, cfg.PopulationSize),
	}
}

func (o *Optimizer) Optimize(ctx context.Context, paramSpaces []ParameterSpace) (*OptimizationResult, error) {
	startTime := time.Now()

	logger.Info("Starting optimization",
		"population", o.config.PopulationSize,
		"generations", o.config.Generations,
		"params", len(paramSpaces),
	)

	o.population = make([]*Individual, 0, o.config.PopulationSize)

	for i := 0; i < o.config.PopulationSize; i++ {
		individual := o.createRandomIndividual(paramSpaces)
		o.population = append(o.population, individual)
	}

	if err := o.evaluatePopulation(ctx, paramSpaces); err != nil {
		return nil, err
	}

	var bestFitness float64 = -math.MaxFloat64
	generationsWithoutImprovement := 0

	for gen := 0; gen < o.config.Generations; gen++ {
		select {
		case <-ctx.Done():
			logger.Info("Optimization cancelled")
			return o.buildResult(gen, startTime, false), nil
		default:
		}

		o.sortPopulation()

		if o.population[0].Fitness > bestFitness {
			bestFitness = o.population[0].Fitness
			generationsWithoutImprovement = 0
		} else {
			generationsWithoutImprovement++
		}

		if generationsWithoutImprovement > 20 {
			logger.Info("Early convergence", "generation", gen)
			return o.buildResult(gen, startTime, true), nil
		}

		newPopulation := make([]*Individual, 0, o.config.PopulationSize)

		for i := 0; i < o.config.EliteCount; i++ {
			elite := o.cloneIndividual(o.population[i])
			newPopulation = append(newPopulation, elite)
		}

		for len(newPopulation) < o.config.PopulationSize {
			var child *Individual

			if rand.Float64() < o.config.CrossoverRate {
				parent1 := o.tournamentSelect()
				parent2 := o.tournamentSelect()
				child = o.crossover(parent1, parent2, paramSpaces)
			} else {
				parent := o.tournamentSelect()
				child = o.cloneIndividual(parent)
			}

			if rand.Float64() < o.config.MutationRate {
				o.mutate(child, paramSpaces)
			}

			newPopulation = append(newPopulation, child)
		}

		o.population = newPopulation

		if err := o.evaluatePopulation(ctx, paramSpaces); err != nil {
			return nil, err
		}

		if gen%10 == 0 {
			best := o.population[0]
			logger.Info("Generation completed",
				"generation", gen,
				"best_fitness", best.Fitness,
				"total_return", best.Metrics.TotalReturn,
				"sharpe", best.Metrics.SharpeRatio,
				"drawdown", best.Metrics.MaxDrawdown,
			)
		}
	}

	return o.buildResult(o.config.Generations, startTime, false), nil
}

func (o *Optimizer) GridSearch(ctx context.Context, paramSpaces []ParameterSpace) (*OptimizationResult, error) {
	startTime := time.Now()

	logger.Info("Starting grid search",
		"params", len(paramSpaces),
	)

	combinations := o.generateAllCombinations(paramSpaces)
	logger.Info("Total combinations", "count", len(combinations))

	results := make([]*Individual, 0, len(combinations))

	for i, combo := range combinations {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		individual := &Individual{
			ID:        fmt.Sprintf("grid_%d", i),
			Genes:     combo,
			CreatedAt: time.Now(),
		}

		result, err := o.strategy.Backtest(combo)
		if err != nil {
			logger.Warn("Backtest failed", "error", err)
			continue
		}

		individual.Metrics = FitnessMetrics{
			TotalReturn:  result.TotalReturn,
			SharpeRatio:  result.SharpeRatio,
			MaxDrawdown:  result.MaxDrawdown,
			WinRate:      result.WinRate,
			ProfitFactor: result.ProfitFactor,
			TotalTrades:  result.TotalTrades,
			AvgTrade:     result.AvgTrade,
		}

		individual.Fitness = o.calculateFitness(&individual.Metrics)

		results = append(results, individual)

		if i%100 == 0 {
			logger.Info("Grid search progress",
				"progress", fmt.Sprintf("%d/%d", i, len(combinations)),
				"best_fitness", results[0].Fitness,
			)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Fitness > results[j].Fitness
	})

	o.population = results

	return o.buildResult(len(combinations), startTime, false), nil
}

func (o *Optimizer) BayesianOptimize(ctx context.Context, paramSpaces []ParameterSpace, initialSamples int) (*OptimizationResult, error) {
	startTime := time.Now()

	logger.Info("Starting Bayesian optimization",
		"initial_samples", initialSamples,
		"generations", o.config.Generations,
	)

	samples := make([]map[string]interface{}, 0)
	sampleResults := make([]float64, 0)

	for i := 0; i < initialSamples && i < o.config.PopulationSize; i++ {
		individual := o.createRandomIndividual(paramSpaces)
		samples = append(samples, individual.Genes)

		result, err := o.strategy.Backtest(individual.Genes)
		if err != nil {
			continue
		}

		metrics := FitnessMetrics{
			TotalReturn:  result.TotalReturn,
			SharpeRatio:  result.SharpeRatio,
			MaxDrawdown:  result.MaxDrawdown,
		}

		sampleResults = append(sampleResults, o.calculateFitness(&metrics))
		o.population = append(o.population, individual)
	}

	for gen := 0; gen < o.config.Generations; gen++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		nextPoint := o.getNextBayesianPoint(samples, sampleResults, paramSpaces)
		samples = append(samples, nextPoint)

		individual := &Individual{
			ID:        fmt.Sprintf("bayesian_%d", gen),
			Genes:     nextPoint,
			CreatedAt: time.Now(),
		}

		result, err := o.strategy.Backtest(nextPoint)
		if err != nil {
			continue
		}

		metrics := FitnessMetrics{
			TotalReturn:  result.TotalReturn,
			SharpeRatio:  result.SharpeRatio,
			MaxDrawdown:  result.MaxDrawdown,
		}

		individual.Metrics = metrics
		individual.Fitness = o.calculateFitness(&metrics)

		sampleResults = append(sampleResults, individual.Fitness)
		o.population = append(o.population, individual)

		if gen%10 == 0 {
			logger.Info("Bayesian iteration",
				"iteration", gen,
				"best_fitness", sampleResults[0],
			)
		}
	}

	sort.Slice(o.population, func(i, j int) bool {
		return o.population[i].Fitness > o.population[j].Fitness
	})

	return o.buildResult(o.config.Generations, startTime, false), nil
}

func (o *Optimizer) createRandomIndividual(paramSpaces []ParameterSpace) *Individual {
	genes := make(map[string]interface{})

	for _, space := range paramSpaces {
		if len(space.Values) > 0 {
			genes[space.Name] = space.Values[rand.IntN(len(space.Values))]
		} else {
			value := space.Min + rand.Float64()*(space.Max-space.Min)
			if space.Step > 0 {
				value = math.Round(value/space.Step) * space.Step
			}
			genes[space.Name] = value
		}
	}

	return &Individual{
		ID:        fmt.Sprintf("ind_%d", rand.Int64()),
		Genes:     genes,
		CreatedAt: time.Now(),
	}
}

func (o *Optimizer) cloneIndividual(original *Individual) *Individual {
	clone := &Individual{
		ID:        fmt.Sprintf("clone_%d", rand.Int64()),
		Genes:     make(map[string]interface{}),
		CreatedAt: time.Now(),
	}

	for k, v := range original.Genes {
		clone.Genes[k] = v
	}

	return clone
}

func (o *Optimizer) crossover(parent1, parent2 *Individual, paramSpaces []ParameterSpace) *Individual {
	child := o.cloneIndividual(parent1)

	for _, space := range paramSpaces {
		if rand.Float64() < 0.5 {
			child.Genes[space.Name] = parent2.Genes[space.Name]
		}
	}

	return child
}

func (o *Optimizer) mutate(individual *Individual, paramSpaces []ParameterSpace) {
	for _, space := range paramSpaces {
		if rand.Float64() < o.config.MutationRate {
			if len(space.Values) > 0 {
				individual.Genes[space.Name] = space.Values[rand.IntN(len(space.Values))]
			} else {
				range_ := space.Max - space.Min
				mutation := (rand.Float64()*2 - 1) * range_ * 0.1

				current, ok := individual.Genes[space.Name].(float64)
				if !ok {
					current = space.Min
				}

				newValue := current + mutation
				if newValue < space.Min {
					newValue = space.Min
				}
				if newValue > space.Max {
					newValue = space.Max
				}

				if space.Step > 0 {
					newValue = math.Round(newValue/space.Step) * space.Step
				}

				individual.Genes[space.Name] = newValue
			}
		}
	}
}

func (o *Optimizer) tournamentSelect() *Individual {
	tournamentSize := 5
	best := o.population[rand.IntN(len(o.population))]

	for i := 1; i < tournamentSize; i++ {
		candidate := o.population[rand.IntN(len(o.population))]
		if candidate.Fitness > best.Fitness {
			best = candidate
		}
	}

	return best
}

func (o *Optimizer) evaluatePopulation(ctx context.Context, paramSpaces []ParameterSpace) error {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, o.config.Workers)
	if o.config.Workers == 0 {
		o.config.Workers = 4
	}

	for i := range o.population {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}

			individual := o.population[idx]

			result, err := o.strategy.Backtest(individual.Genes)
			if err != nil {
				logger.Warn("Backtest failed", "individual", individual.ID, "error", err)
				individual.Fitness = -1000
				return
			}

			individual.Metrics = FitnessMetrics{
				TotalReturn:  result.TotalReturn,
				SharpeRatio: result.SharpeRatio,
				MaxDrawdown: result.MaxDrawdown,
				WinRate:     result.WinRate,
				ProfitFactor: result.ProfitFactor,
				TotalTrades: result.TotalTrades,
				AvgTrade:   result.AvgTrade,
			}

			individual.Fitness = o.calculateFitness(&individual.Metrics)
		}(i)
	}

	wg.Wait()
	return nil
}

func (o *Optimizer) calculateFitness(metrics *FitnessMetrics) float64 {
	switch o.config.Objective {
	case "sharpe":
		return metrics.SharpeRatio

	case "profit":
		return metrics.TotalReturn

	case "calmar":
		return metrics.CalmarRatio

	case "sortino":
		return metrics.SortinoRatio

	default:
		sharpeWeight := 0.4
		drawdownWeight := 0.3
		return sharpeWeight*metrics.SharpeRatio +
			drawdownWeight*(1-metrics.MaxDrawdown) +
			0.15*metrics.TotalReturn +
			0.15*metrics.ProfitFactor
	}
}

func (o *Optimizer) sortPopulation() {
	sort.Slice(o.population, func(i, j int) bool {
		return o.population[i].Fitness > o.population[j].Fitness
	})
}

func (o *Optimizer) buildResult(generations int, startTime time.Time, converged bool) *OptimizationResult {
	o.sortPopulation()

	best := o.population[0]
	if best == nil && len(o.population) > 0 {
		best = o.population[0]
	}

	return &OptimizationResult{
		BestIndividual: best,
		Population:    o.population,
		GenerationsRun: generations,
		TimeTaken:     time.Since(startTime),
		Converged:     converged,
	}
}

func (o *Optimizer) generateAllCombinations(paramSpaces []ParameterSpace) []map[string]interface{} {
	if len(paramSpaces) == 0 {
		return []map[string]interface{}{}
	}

	combinations := []map[string]interface{}{}

	var generate func(idx int, current map[string]interface{})
	generate = func(idx int, current map[string]interface{}) {
		if idx == len(paramSpaces) {
			combo := make(map[string]interface{})
			for k, v := range current {
				combo[k] = v
			}
			combinations = append(combinations, combo)
			return
		}

		space := paramSpaces[idx]

		if len(space.Values) > 0 {
			for _, val := range space.Values {
				current[space.Name] = val
				generate(idx+1, current)
			}
		} else {
			for value := space.Min; value <= space.Max; value += space.Step {
				current[space.Name] = value
				generate(idx+1, current)
			}
		}
	}

	generate(0, make(map[string]interface{}))

	return combinations
}

func (o *Optimizer) getNextBayesianPoint(samples []map[string]interface{}, results []float64, paramSpaces []ParameterSpace) map[string]interface{} {
	next := make(map[string]interface{})

	for _, space := range paramSpaces {
		best := samples[0]
		if space.Step > 0 {
			next[space.Name] = space.Min + rand.Float64()*(space.Max-space.Min)
		} else if len(space.Values) > 0 {
			next[space.Name] = space.Values[rand.IntN(len(space.Values))]
		} else {
			next[space.Name] = space.Min + rand.Float64()*(space.Max-space.Min)
		}

		bestIdx := 0
		for i, r := range results {
			if r > results[bestIdx] {
				bestIdx = i
			}
		}

		_ = best
	}

	return next
}

func (o *Optimizer) GetStatistics() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if len(o.population) == 0 {
		return nil
	}

	var totalFitness, totalReturn, totalSharpe, totalDrawdown float64

	for _, ind := range o.population {
		totalFitness += ind.Fitness
		totalReturn += ind.Metrics.TotalReturn
		totalSharpe += ind.Metrics.SharpeRatio
		totalDrawdown += ind.Metrics.MaxDrawdown
	}

	n := float64(len(o.population))

	return map[string]interface{}{
		"population_size":    len(o.population),
		"avg_fitness":       totalFitness / n,
		"avg_return":        totalReturn / n,
		"avg_sharpe":        totalSharpe / n,
		"avg_drawdown":      totalDrawdown / n,
		"best_fitness":      o.population[0].Fitness,
		"best_return":       o.population[0].Metrics.TotalReturn,
		"worst_fitness":     o.population[len(o.population)-1].Fitness,
	}
}
