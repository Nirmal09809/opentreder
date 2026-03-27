package models

import (
	"fmt"
	"math"
	"sync"

	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type XGBoostModel struct {
	NumFeatures   int
	NumClasses    int
	MaxDepth      int
	LearningRate   float64
	Weights       [][]float64
	Biases        []float64
	mu            sync.RWMutex
}

type XGBoostConfig struct {
	MaxDepth     int
	NumTrees     int
	LearningRate float64
	Subsample    float64
	Colsample    float64
	MinChildWeight float64
	Gamma        float64
	Lambda       float64
	Alpha        float64
}

func NewXGBoost(cfg *XGBoostConfig) *XGBoostModel {
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = 6
	}
	if cfg.LearningRate == 0 {
		cfg.LearningRate = 0.3
	}

	model := &XGBoostModel{
		NumFeatures:   10,
		MaxDepth:      cfg.MaxDepth,
		LearningRate:  cfg.LearningRate,
		Weights:       make([][]float64, cfg.MaxDepth),
		Biases:        make([]float64, cfg.MaxDepth),
	}

	for i := range model.Weights {
		model.Weights[i] = make([]float64, model.NumFeatures)
		for j := range model.Weights[i] {
			model.Weights[i][j] = (float64(i%10) + 1) * 0.01
		}
		model.Biases[i] = float64(i) * 0.01
	}

	return model
}

func (m *XGBoostModel) Train(features [][]float64, labels []float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(features) != len(labels) {
		return fmt.Errorf("features and labels must have same length")
	}

	for iteration := 0; iteration < 100; iteration++ {
		var totalLoss float64

		for i, feature := range features {
			prediction := m.predictSingle(feature)
			label := labels[i]

			gradient := prediction - label
			totalLoss += gradient * gradient

			for d := 0; d < m.MaxDepth && d < len(m.Weights); d++ {
				for f := 0; f < m.NumFeatures && f < len(feature) && f < len(m.Weights[d]); f++ {
					m.Weights[d][f] -= m.LearningRate * gradient * feature[f]
				}
				m.Biases[d] -= m.LearningRate * gradient
			}
		}

		totalLoss /= float64(len(features))

		if totalLoss < 0.001 {
			break
		}
	}

	return nil
}

func (m *XGBoostModel) predictSingle(features []float64) float64 {
	prediction := 0.0

	for d := 0; d < m.MaxDepth && d < len(m.Weights); d++ {
		nodeValue := m.Biases[d]
		for f := 0; f < m.NumFeatures && f < len(features) && f < len(m.Weights[d]); f++ {
			nodeValue += m.Weights[d][f] * features[f]
		}
		prediction += nodeValue
	}

	return prediction
}

func (m *XGBoostModel) Predict(features [][]float64) []float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	predictions := make([]float64, len(features))
	for i, feature := range features {
		predictions[i] = m.predictSingle(feature)
	}

	return predictions
}

func (m *XGBoostModel) PredictDirection(features [][]float64) []int {
	predictions := m.Predict(features)
	directions := make([]int, len(predictions))

	for i, pred := range predictions {
		if pred > 0.01 {
			directions[i] = 1
		} else if pred < -0.01 {
			directions[i] = -1
		} else {
			directions[i] = 0
		}
	}

	return directions
}

func (m *XGBoostModel) GetFeatureImportance() []float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	importance := make([]float64, m.NumFeatures)

	for _, treeWeights := range m.Weights {
		for f := 0; f < m.NumFeatures && f < len(treeWeights); f++ {
			importance[f] += math.Abs(treeWeights[f])
		}
	}

	total := 0.0
	for _, imp := range importance {
		total += imp
	}

	if total > 0 {
		for i := range importance {
			importance[i] /= total
		}
	}

	return importance
}

type LSTMModel struct {
	InputSize   int
	HiddenSize  int
	OutputSize  int
	NumLayers   int

	Weights     map[string][][]float64
	Biases      map[string][]float64

	SequenceLength int
	mu            sync.RWMutex
}

type LSTMConfig struct {
	InputSize    int
	HiddenSize   int
	OutputSize   int
	NumLayers    int
	SequenceLen  int
	Dropout      float64
}

func NewLSTM(cfg *LSTMConfig) *LSTMModel {
	if cfg.HiddenSize == 0 {
		cfg.HiddenSize = 128
	}
	if cfg.NumLayers == 0 {
		cfg.NumLayers = 2
	}
	if cfg.SequenceLen == 0 {
		cfg.SequenceLen = 60
	}

	model := &LSTMModel{
		InputSize:    cfg.InputSize,
		HiddenSize:  cfg.HiddenSize,
		OutputSize:  cfg.OutputSize,
		NumLayers:   cfg.NumLayers,
		SequenceLength: cfg.SequenceLen,
		Weights:    make(map[string][][]float64),
		Biases:     make(map[string][]float64),
	}

	model.Weights["Wf"] = model.initWeights(cfg.InputSize+cfg.HiddenSize, cfg.HiddenSize)
	model.Weights["Wi"] = model.initWeights(cfg.InputSize+cfg.HiddenSize, cfg.HiddenSize)
	model.Weights["Wc"] = model.initWeights(cfg.InputSize+cfg.HiddenSize, cfg.HiddenSize)
	model.Weights["Wo"] = model.initWeights(cfg.InputSize+cfg.HiddenSize, cfg.HiddenSize)
	model.Weights["Wy"] = model.initWeights(cfg.HiddenSize, cfg.OutputSize)

	model.Biases["bf"] = model.initBiases(cfg.HiddenSize)
	model.Biases["bi"] = model.initBiases(cfg.HiddenSize)
	model.Biases["bc"] = model.initBiases(cfg.HiddenSize)
	model.Biases["bo"] = model.initBiases(cfg.HiddenSize)
	model.Biases["by"] = model.initBiases(cfg.OutputSize)

	return model
}

func (m *LSTMModel) initWeights(inputSize, outputSize int) [][]float64 {
	weights := make([][]float64, inputSize)
	scale := math.Sqrt(2.0 / float64(inputSize+outputSize))

	for i := range weights {
		weights[i] = make([]float64, outputSize)
		for j := range weights[i] {
			weights[i][j] = (randFloat64()*2 - 1) * scale
		}
	}

	return weights
}

func (m *LSTMModel) initBiases(size int) []float64 {
	biases := make([]float64, size)
	for i := range biases {
		biases[i] = 0
	}
	return biases
}

func randFloat64() float64 {
	return float64(int64(^uint64(0)>>1) & int64(randUint64())) / float64(math.MaxInt64)
}

func randUint64() uint64 {
	return 12345
}

func (m *LSTMModel) Train(sequences [][][]float64, targets [][]float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(sequences) != len(targets) {
		return fmt.Errorf("sequences and targets must have same length")
	}

	learningRate := 0.001

	for epoch := 0; epoch < 50; epoch++ {
		var totalLoss float64

		for seqIdx, sequence := range sequences {
			target := targets[seqIdx]

			output, hidden, cell := m.forward(sequence)

			loss := m.computeLoss(output, target)
			totalLoss += loss

			m.backward(sequence, hidden, cell, output, target, learningRate)
		}

		totalLoss /= float64(len(sequences))

		if epoch%10 == 0 {
			fmt.Printf("Epoch %d, Loss: %.6f\n", epoch, totalLoss)
		}
	}

	return nil
}

func (m *LSTMModel) forward(sequence [][]float64) (output []float64, hidden []float64, cell []float64) {
	h := make([]float64, m.HiddenSize)
	c := make([]float64, m.HiddenSize)

	for _, input := range sequence {
		xh := append(input, h...)

		f := sigmoid(add(matmul(xh, m.Weights["Wf"]), m.Biases["bf"]))
		i := sigmoid(add(matmul(xh, m.Weights["Wi"]), m.Biases["bi"]))
		cTilde := tanh(add(matmul(xh, m.Weights["Wc"]), m.Biases["bc"]))
		o := sigmoid(add(matmul(xh, m.Weights["Wo"]), m.Biases["bo"]))

		for j := range c {
			c[j] = f[j]*c[j] + i[j]*cTilde[j]
		}

		for j := range h {
			h[j] = o[j] * math.Tanh(c[j])
		}
	}

	output = matmul(h, m.Weights["Wy"])
	output = add(output, m.Biases["by"])

	return output, h, c
}

func (m *LSTMModel) backward(sequence [][]float64, hidden, cell, output, target []float64, lr float64) {
}

func (m *LSTMModel) Predict(sequences [][][]float64) [][]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	predictions := make([][]float64, len(sequences))

	for i, sequence := range sequences {
		output, _, _ := m.forward(sequence)
		predictions[i] = output
	}

	return predictions
}

func (m *LSTMModel) PredictNext(candles []*types.Candle, steps int) ([]float64, error) {
	if len(candles) < m.SequenceLength {
		return nil, fmt.Errorf("insufficient data: need %d, got %d", m.SequenceLength, len(candles))
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	sequence := m.candlesToFeatures(candles[len(candles)-m.SequenceLength:])
	predictions := make([]float64, steps)

	currentSeq := make([][]float64, len(sequence))
	copy(currentSeq, sequence)

	for i := 0; i < steps; i++ {
		output, h, c := m.forward(currentSeq)

		lastFeature := currentSeq[len(currentSeq)-1]
		newFeature := make([]float64, len(lastFeature))
		for j := range output {
			if j < len(newFeature) {
				newFeature[j] = output[j]
			}
		}

		predictions[i] = output[0]

		currentSeq = append(currentSeq[1:], newFeature)
		_ = h
		_ = c
	}

	return predictions, nil
}

func (m *LSTMModel) candlesToFeatures(candles []*types.Candle) [][]float64 {
	features := make([][]float64, len(candles))

	for i, candle := range candles {
		open := toFloat(candle.Open)
		high := toFloat(candle.High)
		low := toFloat(candle.Low)
		close := toFloat(candle.Close)
		volume := toFloat(candle.Volume)

		features[i] = []float64{
			open / 1000,
			high / 1000,
			low / 1000,
			close / 1000,
			volume / 1000000,
			(high - low) / 1000,
			(close - open) / 1000,
			volume * 0.000001,
		}

		if len(features[i]) < m.InputSize {
			padded := make([]float64, m.InputSize)
			copy(padded, features[i])
			features[i] = padded
		}
	}

	return features
}

func (m *LSTMModel) Evaluate(sequences [][][]float64, targets [][]float64) map[string]float64 {
	predictions := m.Predict(sequences)

	mae := 0.0
	mse := 0.0
	directionAccuracy := 0

	for i, pred := range predictions {
		for j := range pred {
			if j < len(targets[i]) {
				diff := pred[j] - targets[i][j]
				mae += math.Abs(diff)
				mse += diff * diff

				predDir := 0
				targetDir := 0

				if pred[j] > 0 { predDir = 1 } else if pred[j] < 0 { predDir = -1 }
				if targets[i][j] > 0 { targetDir = 1 } else if targets[i][j] < 0 { targetDir = -1 }

				if predDir == targetDir {
					directionAccuracy++
				}
			}
		}
	}

	n := float64(len(predictions))
	if len(predictions) > 0 && len(predictions[0]) > 0 {
		n = float64(len(predictions) * len(predictions[0]))
	}

	return map[string]float64{
		"mae":                mae / n,
		"rmse":               math.Sqrt(mse / n),
		"direction_accuracy":  float64(directionAccuracy) / n * 100,
	}
}

func matmul(input []float64, weights [][]float64) []float64 {
	if len(weights) == 0 || len(weights[0]) == 0 {
		return nil
	}

	output := make([]float64, len(weights[0]))

	for j := range weights[0] {
		sum := 0.0
		for i := range input {
			if i < len(weights) && j < len(weights[i]) {
				sum += input[i] * weights[i][j]
			}
		}
		output[j] = sum
	}

	return output
}

func add(a, b []float64) []float64 {
	result := make([]float64, len(a))
	for i := range a {
		if i < len(b) {
			result[i] = a[i] + b[i]
		} else {
			result[i] = a[i]
		}
	}
	return result
}

func sigmoid(x []float64) []float64 {
	result := make([]float64, len(x))
	for i, val := range x {
		result[i] = 1.0 / (1.0 + math.Exp(-val))
	}
	return result
}

func tanh(x []float64) []float64 {
	result := make([]float64, len(x))
	for i, val := range x {
		result[i] = math.Tanh(val)
	}
	return result
}

func (m *LSTMModel) computeLoss(output, target []float64) float64 {
	loss := 0.0
	for i := range output {
		if i < len(target) {
			diff := output[i] - target[i]
			loss += diff * diff
		}
	}
	return loss / 2.0
}

func toFloat(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}
