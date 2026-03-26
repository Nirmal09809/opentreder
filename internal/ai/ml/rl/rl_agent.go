package ml

import (
	"fmt"
	"math"
	"math/rand"
	"sync"

	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
)

type RLAgent struct {
	config       *RLConfig
	qNetwork     *QNetwork
	targetNetwork *QNetwork
	memory       *ReplayBuffer
	optimizer    *AdamOptimizer
	mu           sync.RWMutex
	epsilon      float64
	steps        int64
}

type RLConfig struct {
	StateDim      int
	ActionDim     int
	HiddenDim     int
	LearningRate float64
	Gamma        float64
	Epsilon      float64
	EpsilonDecay float64
	EpsilonMin   float64
	BatchSize    int
	MemorySize   int
	TargetUpdate int
	MaxSteps     int64
}

type QNetwork struct {
	inputDim   int
	hiddenDim  int
	outputDim  int
	weights1   [][]float64
	weights2   [][]float64
	biases1    []float64
	biases2    []float64
	mu         sync.RWMutex
}

type ReplayBuffer struct {
	capacity int
	buffer   []Transition
	head     int
	size     int
	mu       sync.RWMutex
}

type Transition struct {
	State    []float64
	Action   int
	Reward   float64
	NextState []float64
	Done     bool
}

type AdamOptimizer struct {
	learningRate float64
	beta1       float64
	beta2       float64
	epsilon     float64
	m          [][]float64
	v          [][]float64
	t          int
}

type RLAction int

const (
	ActionHold RLAction = iota
	ActionBuy
	ActionSell
	ActionClose
)

type RLState struct {
	Price      float64
	Position   float64
	Balance    float64
	MA5        float64
	MA20       float64
	RSI        float64
	MACD       float64
	Volume     float64
	Volatility float64
	Momentum   float64
}

func NewRLAgent(cfg *RLConfig) *RLAgent {
	if cfg.LearningRate == 0 {
		cfg.LearningRate = 0.001
	}
	if cfg.Gamma == 0 {
		cfg.Gamma = 0.99
	}
	if cfg.Epsilon == 0 {
		cfg.Epsilon = 1.0
	}
	if cfg.EpsilonDecay == 0 {
		cfg.EpsilonDecay = 0.995
	}
	if cfg.EpsilonMin == 0 {
		cfg.EpsilonMin = 0.01
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 32
	}
	if cfg.MemorySize == 0 {
		cfg.MemorySize = 100000
	}
	if cfg.TargetUpdate == 0 {
		cfg.TargetUpdate = 1000
	}

	qNetwork := NewQNetwork(cfg.StateDim, cfg.HiddenDim, cfg.ActionDim)
	targetNetwork := NewQNetwork(cfg.StateDim, cfg.HiddenDim, cfg.ActionDim)
	targetNetwork.copyWeights(qNetwork)

	return &RLAgent{
		config:       cfg,
		qNetwork:     qNetwork,
		targetNetwork: targetNetwork,
		memory:       NewReplayBuffer(cfg.MemorySize),
		optimizer:    NewAdamOptimizer(cfg.LearningRate),
		epsilon:      cfg.Epsilon,
		steps:        0,
	}
}

func NewQNetwork(inputDim, hiddenDim, outputDim int) *QNetwork {
	network := &QNetwork{
		inputDim:  inputDim,
		hiddenDim: hiddenDim,
		outputDim: outputDim,
		weights1:  initWeightMatrix(inputDim, hiddenDim),
		weights2:  initWeightMatrix(hiddenDim, outputDim),
		biases1:  make([]float64, hiddenDim),
		biases2:  make([]float64, outputDim),
	}
	return network
}

func initWeightMatrix(rows, cols int) [][]float64 {
	matrix := make([][]float64, rows)
	scale := math.Sqrt(2.0 / float64(rows+cols))
	for i := range matrix {
		matrix[i] = make([]float64, cols)
		for j := range matrix[i] {
			matrix[i][j] = rand.Float64()*2*scale - scale
		}
	}
	return matrix
}

func (n *QNetwork) Forward(state []float64) []float64 {
	hidden := n.relu(n.matmul(state, n.weights1))
	hidden = n.add(hidden, n.biases1)
	output := n.matmul(hidden, n.weights2)
	output = n.add(output, n.biases2)
	return output
}

func (n *QNetwork) relu(x []float64) []float64 {
	result := make([]float64, len(x))
	for i, v := range x {
		if v > 0 {
			result[i] = v
		}
	}
	return result
}

func (n *QNetwork) matmul(vec []float64, mat [][]float64) []float64 {
	result := make([]float64, len(mat[0]))
	for j := range mat[0] {
		sum := 0.0
		for i := range vec {
			sum += vec[i] * mat[i][j]
		}
		result[j] = sum
	}
	return result
}

func (n *QNetwork) add(a, b []float64) []float64 {
	result := make([]float64, len(a))
	for i := range a {
		result[i] = a[i] + b[i]
	}
	return result
}

func (n *QNetwork) copyWeights(from *QNetwork) {
	n.mu.Lock()
	defer n.mu.Unlock()

	for i := range n.weights1 {
		for j := range n.weights1[i] {
			n.weights1[i][j] = from.weights1[i][j]
		}
	}

	for i := range n.weights2 {
		for j := range n.weights2[i] {
			n.weights2[i][j] = from.weights2[i][j]
		}
	}

	for i := range n.biases1 {
		n.biases1[i] = from.biases1[i]
	}

	for i := range n.biases2 {
		n.biases2[i] = from.biases2[i]
	}
}

func (n *QNetwork) softUpdate(from *QNetwork, tau float64) {
	n.mu.Lock()
	defer n.mu.Unlock()

	for i := range n.weights1 {
		for j := range n.weights1[i] {
			n.weights1[i][j] = n.weights1[i][j]*(1-tau) + from.weights1[i][j]*tau
		}
	}

	for i := range n.weights2 {
		for j := range n.weights2[i] {
			n.weights2[i][j] = n.weights2[i][j]*(1-tau) + from.weights2[i][j]*tau
		}
	}
}

func NewReplayBuffer(capacity int) *ReplayBuffer {
	return &ReplayBuffer{
		capacity: capacity,
		buffer:   make([]Transition, capacity),
	}
}

func (rb *ReplayBuffer) Push(t Transition) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buffer[rb.head] = t
	rb.head = (rb.head + 1) % rb.capacity
	if rb.size < rb.capacity {
		rb.size++
	}
}

func (rb *ReplayBuffer) Sample(batchSize int) []Transition {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.size < batchSize {
		return nil
	}

	indices := make([]int, batchSize)
	for i := range indices {
		indices[i] = rand.Intn(rb.size)
	}

	sample := make([]Transition, batchSize)
	for i, idx := range indices {
		sample[i] = rb.buffer[idx]
	}

	return sample
}

func NewAdamOptimizer(lr float64) *AdamOptimizer {
	return &AdamOptimizer{
		learningRate: lr,
		beta1:       0.9,
		beta2:       0.999,
		epsilon:     1e-8,
		m:          nil,
		v:          nil,
		t:          0,
	}
}

func (o *AdamOptimizer) Step(gradients [][]float64) [][]float64 {
	o.t++

	if o.m == nil {
		o.m = make([][]float64, len(gradients))
		o.v = make([][]float64, len(gradients))
		for i := range o.m {
			o.m[i] = make([]float64, len(gradients[i]))
			o.v[i] = make([]float64, len(gradients[i]))
		}
	}

	lrHat := o.learningRate * math.Sqrt(1-math.Pow(o.beta2, float64(o.t))) / (1 - math.Pow(o.beta1, float64(o.t)))

	updated := make([][]float64, len(gradients))
	for i := range gradients {
		updated[i] = make([]float64, len(gradients[i]))
		for j := range gradients[i] {
			o.m[i][j] = o.beta1*o.m[i][j] + (1-o.beta1)*gradients[i][j]
			o.v[i][j] = o.beta2*o.v[i][j] + (1-o.beta2)*gradients[i][j]*gradients[i][j]
			updated[i][j] = gradients[i][j] - lrHat*o.m[i][j]/(math.Sqrt(o.v[i][j])+o.epsilon)
		}
	}

	return updated
}

func (a *RLAgent) SelectAction(state []float64) int {
	a.mu.Lock()
	defer a.mu.Unlock()

	if rand.Float64() < a.epsilon {
		return rand.Intn(a.config.ActionDim)
	}

	qValues := a.qNetwork.Forward(state)
	bestAction := 0
	for i := 1; i < len(qValues); i++ {
		if qValues[i] > qValues[bestAction] {
			bestAction = i
		}
	}

	return bestAction
}

func (a *RLAgent) Store(state, nextState []float64, action int, reward float64, done bool) {
	transition := Transition{
		State:     state,
		Action:    action,
		Reward:    reward,
		NextState: nextState,
		Done:     done,
	}
	a.memory.Push(transition)
}

func (a *RLAgent) Learn() {
	batch := a.memory.Sample(a.config.BatchSize)
	if batch == nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	states := make([][]float64, len(batch))
	nextStates := make([][]float64, len(batch))
	actions := make([]int, len(batch))
	rewards := make([]float64, len(batch))
	dones := make([]bool, len(batch))

	for i, t := range batch {
		states[i] = t.State
		nextStates[i] = t.NextState
		actions[i] = t.Action
		rewards[i] = t.Reward
		dones[i] = t.Done
	}

	currentQ := a.qNetwork.ForwardBatch(states)
	for i, action := range actions {
		if dones[i] {
			currentQ[i][action] = rewards[i]
		} else {
			nextQ := a.targetNetwork.Forward(nextStates[i])
			maxNextQ := nextQ[0]
			for j := 1; j < len(nextQ); j++ {
				if nextQ[j] > maxNextQ {
					maxNextQ = nextQ[j]
				}
			}
			currentQ[i][action] = rewards[i] + a.config.Gamma*maxNextQ
		}
	}

	targets := a.computeTargets(batch, currentQ)
	gradients := a.computeGradients(states, targets)
	updatedWeights := a.optimizer.Step(gradients)
	a.applyGradients(updatedWeights)

	a.steps++

	if a.epsilon > a.config.EpsilonMin {
		a.epsilon *= a.config.EpsilonDecay
	}

	if a.steps%int64(a.config.TargetUpdate) == 0 {
		a.targetNetwork.copyWeights(a.qNetwork)
	}

	if a.steps%10000 == 0 {
		logger.Info("RL Agent update",
			"steps", a.steps,
			"epsilon", fmt.Sprintf("%.4f", a.epsilon),
			"memory", a.memory.size,
		)
	}
}

func (n *QNetwork) ForwardBatch(states [][]float64) [][]float64 {
	result := make([][]float64, len(states))
	for i, state := range states {
		result[i] = n.Forward(state)
	}
	return result
}

func (a *RLAgent) computeTargets(batch []Transition, currentQ [][]float64) [][]float64 {
	targets := make([][]float64, len(batch))
	for i := range targets {
		targets[i] = make([]float64, a.config.ActionDim)
		for j := range targets[i] {
			targets[i][j] = currentQ[i][j]
		}
	}
	return targets
}

func (a *RLAgent) computeGradients(states, targets [][]float64) [][]float64 {
	return a.qNetwork.weights1
}

func (a *RLAgent) applyGradients(weights [][]float64) {
	a.qNetwork.mu.Lock()
	defer a.qNetwork.mu.Unlock()

	for i := range a.qNetwork.weights1 {
		for j := range a.qNetwork.weights1[i] {
			if i < len(weights) && j < len(weights[i]) {
				a.qNetwork.weights1[i][j] = weights[i][j]
			}
		}
	}
}

func (a *RLAgent) GetActionName(action int) string {
	switch action {
	case 0:
		return "HOLD"
	case 1:
		return "BUY"
	case 2:
		return "SELL"
	case 3:
		return "CLOSE"
	default:
		return "UNKNOWN"
	}
}

func (a *RLAgent) GetStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"steps":         a.steps,
		"epsilon":       a.epsilon,
		"memory_size":   a.memory.size,
		"memory_capacity": a.memory.capacity,
	}
}

func (a *RLAgent) StateFromCandles(candles []*types.Candle, position, balance float64) []float64 {
	if len(candles) < 20 {
		return make([]float64, a.config.StateDim)
	}

	state := make([]float64, a.config.StateDim)

	state[0] = candles[len(candles)-1].Close.InexactFloat64() / 1000
	state[1] = position / 10000
	state[2] = balance / 1000000

	state[3] = a.calculateSMA(candles, 5) / 1000
	state[4] = a.calculateSMA(candles, 20) / 1000
	state[5] = a.calculateRSI(candles, 14) / 100
	state[6] = a.calculateMACD(candles) / 1000
	state[7] = candles[len(candles)-1].Volume.InexactFloat64() / 1000000
	state[8] = a.calculateVolatility(candles, 20) / 1000
	state[9] = a.calculateMomentum(candles, 10) / 1000

	return state
}

func (a *RLAgent) calculateSMA(candles []*types.Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}

	sum := 0.0
	for i := len(candles) - period; i < len(candles); i++ {
		sum += candles[i].Close.InexactFloat64()
	}

	return sum / float64(period)
}

func (a *RLAgent) calculateRSI(candles []*types.Candle, period int) float64 {
	if len(candles) < period+1 {
		return 50
	}

	var gains, losses float64
	for i := len(candles) - period; i < len(candles); i++ {
		change := candles[i].Close.Sub(candles[i-1].Close).InexactFloat64()
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

func (a *RLAgent) calculateMACD(candles []*types.Candle) float64 {
	if len(candles) < 26 {
		return 0
	}

	ema12 := a.calculateEMA(candles, 12)
	ema26 := a.calculateEMA(candles, 26)

	return ema12 - ema26
}

func (a *RLAgent) calculateEMA(candles []*types.Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}

	multiplier := 2.0 / float64(period+1)
	ema := a.calculateSMA(candles[:period], period)

	for i := period; i < len(candles); i++ {
		ema = (candles[i].Close.InexactFloat64()-ema)*multiplier + ema
	}

	return ema
}

func (a *RLAgent) calculateVolatility(candles []*types.Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}

	mean := a.calculateSMA(candles, period)
	sumSquares := 0.0

	for i := len(candles) - period; i < len(candles); i++ {
		diff := candles[i].Close.InexactFloat64() - mean
		sumSquares += diff * diff
	}

	return math.Sqrt(sumSquares / float64(period))
}

func (a *RLAgent) calculateMomentum(candles []*types.Candle, period int) float64 {
	if len(candles) < period+1 {
		return 0
	}

	recent := candles[len(candles)-1].Close.InexactFloat64()
	old := candles[len(candles)-period-1].Close.InexactFloat64()

	return recent - old
}

type PolicyGradientAgent struct {
	config     *PGConfig
	network   *PolicyNetwork
	optimizer *AdamOptimizer
	mu        sync.RWMutex
}

type PGConfig struct {
	StateDim      int
	ActionDim     int
	HiddenDim     int
	LearningRate float64
	Gamma        float64
}

type PolicyNetwork struct {
	inputDim  int
	hiddenDim int
	outputDim int
	weights1 [][]float64
	weights2 [][]float64
	biases1  []float64
	biases2  []float64
	mu       sync.RWMutex
}

func NewPolicyGradientAgent(cfg *PGConfig) *PolicyGradientAgent {
	if cfg.LearningRate == 0 {
		cfg.LearningRate = 0.0003
	}
	if cfg.Gamma == 0 {
		cfg.Gamma = 0.99
	}

	return &PolicyGradientAgent{
		config:     cfg,
		network:   NewPolicyNetwork(cfg.StateDim, cfg.HiddenDim, cfg.ActionDim),
		optimizer: NewAdamOptimizer(cfg.LearningRate),
	}
}

func NewPolicyNetwork(inputDim, hiddenDim, outputDim int) *PolicyNetwork {
	return &PolicyNetwork{
		inputDim:  inputDim,
		hiddenDim: hiddenDim,
		outputDim: outputDim,
		weights1: initWeightMatrix(inputDim, hiddenDim),
		weights2: initWeightMatrix(hiddenDim, outputDim),
		biases1:  make([]float64, hiddenDim),
		biases2:  make([]float64, outputDim),
	}
}

func (p *PolicyNetwork) Forward(state []float64) []float64 {
	hidden := make([]float64, p.hiddenDim)
	for j := range hidden {
		sum := p.biases1[j]
		for i := range state {
			sum += state[i] * p.weights1[i][j]
		}
		hidden[j] = math.Tanh(sum)
	}

	logits := make([]float64, p.outputDim)
	for j := range logits {
		sum := p.biases2[j]
		for i := range hidden {
			sum += hidden[i] * p.weights2[i][j]
		}
		logits[j] = sum
	}

	probs := softmax(logits)
	return probs
}

func softmax(x []float64) []float64 {
	maxX := x[0]
	for _, v := range x {
		if v > maxX {
			maxX = v
		}
	}

	sum := 0.0
	for _, v := range x {
		sum += math.Exp(v - maxX)
	}

	result := make([]float64, len(x))
	for i, v := range x {
		result[i] = math.Exp(v-maxX) / sum
	}

	return result
}

func (a *PolicyGradientAgent) SelectAction(state []float64) int {
	probs := a.network.Forward(state)
	
	r := rand.Float64()
	cumulative := 0.0
	for i, p := range probs {
		cumulative += p
		if r < cumulative {
			return i
		}
	}

	return len(probs) - 1
}

func (a *PolicyGradientAgent) Learn(trajectory []Transition, advantages []float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, t := range trajectory {
		probs := a.network.Forward(t.State)
		logProb := math.Log(probs[t.Action] + 1e-8)
		policyLoss := -logProb * advantages[i]

		_ = policyLoss
	}
}

type DDPGAgent struct {
	actor        *ActorNetwork
	critic       *CriticNetwork
	targetActor  *ActorNetwork
	targetCritic *CriticNetwork
	config       *DDPGConfig
	memory       *ReplayBuffer
	mu           sync.RWMutex
}

type DDPGConfig struct {
	StateDim     int
	ActionDim    int
	ActorHidden  int
	CriticHidden int
	LearningRate float64
	Gamma        float64
	Tau          float64
}

type ActorNetwork struct {
	inputDim  int
	hiddenDim int
	actionDim int
	weights1 [][]float64
	weights2 [][]float64
	biases1  []float64
	biases2  []float64
}

type CriticNetwork struct {
	stateDim  int
	actionDim int
	hiddenDim int
	weights1 [][]float64
	weights2 [][]float64
	weights3 [][]float64
	biases1  []float64
	biases2  []float64
	biases3  []float64
}

func NewDDPGAgent(cfg *DDPGConfig) *DDPGAgent {
	return &DDPGAgent{
		actor:       NewActorNetwork(cfg.StateDim, cfg.ActorHidden, cfg.ActionDim),
		critic:      NewCriticNetwork(cfg.StateDim, cfg.ActionDim, cfg.CriticHidden),
		targetActor:  NewActorNetwork(cfg.StateDim, cfg.ActorHidden, cfg.ActionDim),
		targetCritic: NewCriticNetwork(cfg.StateDim, cfg.ActionDim, cfg.CriticHidden),
		config:      cfg,
		memory:      NewReplayBuffer(100000),
	}
}

func NewActorNetwork(stateDim, hiddenDim, actionDim int) *ActorNetwork {
	return &ActorNetwork{
		inputDim:  stateDim,
		hiddenDim: hiddenDim,
		actionDim: actionDim,
		weights1: initWeightMatrix(stateDim, hiddenDim),
		weights2: initWeightMatrix(hiddenDim, actionDim),
		biases1:  make([]float64, hiddenDim),
		biases2:  make([]float64, actionDim),
	}
}

func NewCriticNetwork(stateDim, actionDim, hiddenDim int) *CriticNetwork {
	return &CriticNetwork{
		stateDim:  stateDim,
		actionDim: actionDim,
		hiddenDim: hiddenDim,
		weights1: initWeightMatrix(stateDim, hiddenDim),
		weights2: initWeightMatrix(actionDim, hiddenDim),
		weights3: initWeightMatrix(hiddenDim, 1),
		biases1:  make([]float64, hiddenDim),
		biases2:  make([]float64, hiddenDim),
		biases3:  make([]float64, 1),
	}
}
