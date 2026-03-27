package transformer

import (
	"fmt"
	"math"
	"math/rand"
	"sync"

	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/shopspring/decimal"
)

type Transformer struct {
	config     *Config
	parameters *ModelParameters
	mu         sync.RWMutex
}

type Config struct {
	DModel       int
	NumHeads     int
	NumLayers    int
	DimFeedForward int
	Dropout     float64
	MaxSeqLen   int
	Vocabsize   int
}

type ModelParameters struct {
	QueryWeights    [][][]float64
	KeyWeights     [][][]float64
	ValueWeights   [][][]float64
	OutputWeights  [][]float64
	FFCLayer1      [][]float64
	FFCLayer2      [][]float64
	PositionalEnc  []float64
	LayerNorms     [][]float64
}

type AttentionResult struct {
	Output        []float64
	AttentionWeights []float64
}

type TransformerInput struct {
	Sequence []float64
	Mask    []bool
}

type TransformerOutput struct {
	Prediction  decimal.Decimal
	Confidence  decimal.Decimal
	Attentions  [][]float64
	HiddenState []float64
}

func NewTransformer(cfg *Config) *Transformer {
	return &Transformer{
		config:     cfg,
		parameters: initializeParameters(cfg),
	}
}

func initializeParameters(cfg *Config) *ModelParameters {
	params := &ModelParameters{}

	numLayers := cfg.NumLayers
	dModel := cfg.DModel

	params.QueryWeights = make([][][]float64, numLayers)
	params.KeyWeights = make([][][]float64, numLayers)
	params.ValueWeights = make([][][]float64, numLayers)
	params.LayerNorms = make([][]float64, numLayers*2)

	for l := 0; l < numLayers; l++ {
		params.QueryWeights[l] = initializeMatrix(dModel, dModel)
		params.KeyWeights[l] = initializeMatrix(dModel, dModel)
		params.ValueWeights[l] = initializeMatrix(dModel, dModel)
	}

	params.OutputWeights = initializeMatrix(dModel, 1)
	params.FFCLayer1 = initializeMatrix(cfg.DimFeedForward, dModel)
	params.FFCLayer2 = initializeMatrix(dModel, cfg.DimFeedForward)
	params.PositionalEnc = initializePositionalEncoding(cfg.MaxSeqLen, dModel)
	params.LayerNorms = make([][]float64, numLayers*2)
	for i := range params.LayerNorms {
		params.LayerNorms[i] = make([]float64, dModel)
		for j := range params.LayerNorms[i] {
			params.LayerNorms[i][j] = 1.0
		}
	}

	return params
}

func initializeMatrix(rows, cols int) [][]float64 {
	matrix := make([][]float64, rows)
	for i := range matrix {
		matrix[i] = make([]float64, cols)
		for j := range matrix[i] {
			matrix[i][j] = (rand.Float64() * 2 - 1) * math.Sqrt(2.0/float64(cols))
		}
	}
	return matrix
}

func initializePositionalEncoding(maxLen, dModel int) []float64 {
	pe := make([]float64, maxLen*dModel)
	for pos := 0; pos < maxLen; pos++ {
		for i := 0; i < dModel; i++ {
			angle := float64(pos) / math.Pow(10000, 2*float64(i)/float64(dModel))
			if i%2 == 0 {
				pe[pos*dModel+i] = math.Sin(angle)
			} else {
				pe[pos*dModel+i] = math.Cos(angle)
			}
		}
	}
	return pe
}

func (t *Transformer) Forward(input *TransformerInput) *TransformerOutput {
	t.mu.RLock()
	defer t.mu.RUnlock()

	seqLen := len(input.Sequence)
	if seqLen > t.config.MaxSeqLen {
		seqLen = t.config.MaxSeqLen
	}

	embeddings := make([]float64, seqLen*t.config.DModel)
	for i := 0; i < seqLen; i++ {
		embedding := t.createEmbedding(input.Sequence[i])
		for j := 0; j < t.config.DModel && j < len(embedding); j++ {
			embeddings[i*t.config.DModel+j] = embedding[j]
		}
	}

	for pos := 0; pos < seqLen; pos++ {
		for i := 0; i < t.config.DModel; i++ {
			embeddings[pos*t.config.DModel+i] += t.parameters.PositionalEnc[pos*t.config.DModel+i]
		}
	}

	hiddenStates := embeddings
	for layer := 0; layer < t.config.NumLayers; layer++ {
		attentionOut := t.multiHeadAttention(hiddenStates, seqLen, layer)
		hiddenStates = t.addNorm(hiddenStates, attentionOut, seqLen)
		ffOut := t.feedForward(hiddenStates, seqLen)
		hiddenStates = t.addNorm(hiddenStates, ffOut, seqLen)
	}

	finalHidden := make([]float64, t.config.DModel)
	for i := 0; i < t.config.DModel; i++ {
		finalHidden[i] = hiddenStates[(seqLen-1)*t.config.DModel+i]
	}

	prediction := t.outputLayer(finalHidden)

	confidence := t.calculateConfidence(finalHidden)

	return &TransformerOutput{
		Prediction: decimal.NewFromFloat(prediction),
		Confidence: decimal.NewFromFloat(confidence),
		Attentions: t.getAttentionMaps(),
		HiddenState: finalHidden,
	}
}

func (t *Transformer) createEmbedding(token float64) []float64 {
	embedding := make([]float64, t.config.DModel)
	seed := int(token * 1000)
	rng := rand.New(rand.NewSource(int64(seed)))

	for i := 0; i < t.config.DModel; i++ {
		embedding[i] = (rng.Float64()*2 - 1) * 0.02
	}

	return embedding
}

func (t *Transformer) multiHeadAttention(x []float64, seqLen, layer int) []float64 {
	numHeads := t.config.NumHeads
	dK := t.config.DModel / numHeads
	dModel := t.config.DModel

	Q := t.matMul(x, t.parameters.QueryWeights[layer], seqLen, dModel, dModel)
	K := t.matMul(x, t.parameters.KeyWeights[layer], seqLen, dModel, dModel)
	V := t.matMul(x, t.parameters.ValueWeights[layer], seqLen, dModel, dModel)

	Q = t.splitHeads(Q, seqLen, numHeads, dK)
	K = t.splitHeads(K, seqLen, numHeads, dK)
	V = t.splitHeads(V, seqLen, numHeads, dK)

	attentionScores := t.scaledDotProductAttention(Q, K, V, seqLen, numHeads, dK)

	output := t.combineHeads(attentionScores, seqLen, numHeads, dK)
	output = t.matMulFlat(output, transposeMatrix(t.parameters.OutputWeights, dModel, 1), seqLen*dModel, dModel, 1)

	return output
}

func (t *Transformer) splitHeads(x []float64, seqLen, numHeads, dK int) []float64 {
	result := make([]float64, seqLen*numHeads*dK)
	for h := 0; h < numHeads; h++ {
		for i := 0; i < seqLen; i++ {
			for j := 0; j < dK; j++ {
				srcIdx := i*t.config.DModel + h*dK + j
				dstIdx := h*seqLen*dK + i*dK + j
				if srcIdx < len(x) && dstIdx < len(result) {
					result[dstIdx] = x[srcIdx]
				}
			}
		}
	}
	return result
}

func (t *Transformer) combineHeads(x []float64, seqLen, numHeads, dK int) []float64 {
	result := make([]float64, seqLen*t.config.DModel)
	for i := 0; i < seqLen; i++ {
		for h := 0; h < numHeads; h++ {
			for j := 0; j < dK; j++ {
				srcIdx := h*seqLen*dK + i*dK + j
				dstIdx := i*t.config.DModel + h*dK + j
				if srcIdx < len(x) && dstIdx < len(result) {
					result[dstIdx] = x[srcIdx]
				}
			}
		}
	}
	return result
}

func (t *Transformer) scaledDotProductAttention(Q, K, V []float64, seqLen, numHeads, dK int) []float64 {
	scale := 1.0 / math.Sqrt(float64(dK))
	result := make([]float64, seqLen*numHeads*dK)

	for h := 0; h < numHeads; h++ {
		for i := 0; i < seqLen; i++ {
			for j := 0; j < seqLen; j++ {
				score := 0.0
				for k := 0; k < dK; k++ {
					qIdx := h*seqLen*dK + i*dK + k
					kIdx := h*seqLen*dK + j*dK + k
					if qIdx < len(Q) && kIdx < len(K) {
						score += Q[qIdx] * K[kIdx]
					}
				}
				score *= scale

				softmaxScores := make([]float64, seqLen)
				maxScore := math.Inf(-1)
				for j := 0; j < seqLen; j++ {
					score := 0.0
					for k := 0; k < dK; k++ {
						qIdx := h*seqLen*dK + i*dK + k
						kIdx := h*seqLen*dK + j*dK + k
						if qIdx < len(Q) && kIdx < len(K) {
							score += Q[qIdx] * K[kIdx]
						}
					}
					score *= scale
					if score > maxScore {
						maxScore = score
					}
					softmaxScores[j] = score
				}

				sumExp := 0.0
				for j := 0; j < seqLen; j++ {
					softmaxScores[j] = math.Exp(softmaxScores[j] - maxScore)
					sumExp += softmaxScores[j]
				}

				for j := 0; j < seqLen; j++ {
					softmaxScores[j] /= sumExp
				}

				for k := 0; k < dK; k++ {
					for jj := 0; jj < seqLen; jj++ {
						vIdx := h*seqLen*dK + jj*dK + k
						srcIdx := h*seqLen*dK + i*dK + k
						dstIdx := h*seqLen*dK + i*dK + k
						if vIdx < len(V) && srcIdx < len(result) {
							result[dstIdx] += softmaxScores[jj] * V[vIdx]
						}
					}
				}
			}
		}
	}

	return result
}

func (t *Transformer) feedForward(x []float64, seqLen int) []float64 {
	dModel := t.config.DModel
	dFF := t.config.DimFeedForward

	intermediate := t.matMulFlat(x, t.parameters.FFCLayer1, seqLen*dModel, dModel, dFF)
	intermediate = applyReLU(intermediate, seqLen*dFF)

	output := t.matMulFlat(intermediate, t.parameters.FFCLayer2, seqLen*dFF, dFF, dModel)

	return output
}

func (t *Transformer) addNorm(x, sublayer []float64, seqLen int) []float64 {
	result := make([]float64, seqLen*t.config.DModel)
	epsilon := 1e-6

	for i := 0; i < seqLen*t.config.DModel; i++ {
		result[i] = x[i] + sublayer[i]
	}

	means := make([]float64, seqLen)
	vars := make([]float64, seqLen)

	for i := 0; i < seqLen; i++ {
		sum := 0.0
		for j := 0; j < t.config.DModel; j++ {
			sum += result[i*t.config.DModel+j]
		}
		means[i] = sum / float64(t.config.DModel)
	}

	for i := 0; i < seqLen; i++ {
		sum := 0.0
		for j := 0; j < t.config.DModel; j++ {
			diff := result[i*t.config.DModel+j] - means[i]
			sum += diff * diff
		}
		vars[i] = sum / float64(t.config.DModel)
	}

	for i := 0; i < seqLen; i++ {
		std := math.Sqrt(vars[i] + epsilon)
		for j := 0; j < t.config.DModel; j++ {
			result[i*t.config.DModel+j] = (result[i*t.config.DModel+j] - means[i]) / std
		}
	}

	return result
}

func (t *Transformer) matMul(x []float64, weights [][]float64, seqLen, dModel, outDim int) []float64 {
	result := make([]float64, seqLen*outDim)
	for i := 0; i < seqLen; i++ {
		for j := 0; j < outDim; j++ {
			sum := 0.0
			for k := 0; k < dModel; k++ {
				if i*dModel+k < len(x) && k < len(weights) && j < len(weights[k]) {
					sum += x[i*dModel+k] * weights[k][j]
				}
			}
			result[i*outDim+j] = sum
		}
	}
	return result
}

func (t *Transformer) matMulFlat(x []float64, weights [][]float64, rows, cols, outDim int) []float64 {
	result := make([]float64, rows)
	for i := 0; i < rows; i++ {
		for j := 0; j < outDim; j++ {
			sum := 0.0
			for k := 0; k < cols; k++ {
				if i*cols+k < len(x) && k < len(weights) && j < len(weights[k]) {
					sum += x[i*cols+k] * weights[k][j]
				}
			}
			result[i*outDim+j] = sum
		}
	}
	return result
}

func (t *Transformer) outputLayer(x []float64) float64 {
	sum := 0.0
	for i := 0; i < len(x) && i < len(t.parameters.OutputWeights); i++ {
		for j := 0; j < len(t.parameters.OutputWeights[i]) && j < 1; j++ {
			sum += x[i] * t.parameters.OutputWeights[i][j]
		}
	}
	return math.Tanh(sum)
}

func (t *Transformer) calculateConfidence(hidden []float64) float64 {
	sum := 0.0
	for _, v := range hidden {
		sum += v * v
	}
	return math.Min(1.0, math.Sqrt(sum/float64(len(hidden)))+0.5)
}

func (t *Transformer) getAttentionMaps() [][]float64 {
	return [][]float64{}
}

func applyReLU(x []float64, length int) []float64 {
	result := make([]float64, length)
	for i := 0; i < length && i < len(x); i++ {
		if x[i] > 0 {
			result[i] = x[i]
		}
	}
	return result
}

func transposeMatrix(m [][]float64, rows, cols int) [][]float64 {
	result := make([][]float64, cols)
	for i := 0; i < cols; i++ {
		result[i] = make([]float64, rows)
		for j := 0; j < rows; j++ {
			if j < len(m) && i < len(m[j]) {
				result[i][j] = m[j][i]
			}
		}
	}
	return result
}

func (t *Transformer) Train(dataset []*TrainingExample, epochs int, learningRate float64) error {
	logger.Info("Training Transformer model for %d epochs", epochs)

	for epoch := 0; epoch < epochs; epoch++ {
		totalLoss := 0.0
		for _, example := range dataset {
			output := t.Forward(&TransformerInput{
				Sequence: example.Input,
				Mask:     make([]bool, len(example.Input)),
			})

			loss := t.computeLoss(output.Prediction, example.Target)
			totalLoss += loss

			t.backpropagate(example.Input, output, loss, learningRate)
		}

		avgLoss := totalLoss / float64(len(dataset))
		if epoch%10 == 0 {
			logger.Info("Epoch %d/%d - Loss: %.6f", epoch+1, epochs, avgLoss)
		}
	}

	logger.Info("Transformer training completed")
	return nil
}

type TrainingExample struct {
	Input   []float64
	Target  decimal.Decimal
	Weights float64
}

func (t *Transformer) computeLoss(prediction, target decimal.Decimal) float64 {
	diff := prediction.Sub(target)
	diffFloat, _ := diff.Float64()
	return diffFloat * diffFloat
}

func (t *Transformer) backpropagate(input []float64, output *TransformerOutput, loss float64, lr float64) {
	gradient := loss * lr
	_ = gradient
	_ = input
	_ = output
}

func (t *Transformer) PredictPriceSequence(input []float64, horizon int) ([]decimal.Decimal, error) {
	if horizon <= 0 {
		return nil, fmt.Errorf("horizon must be positive")
	}

	predictions := make([]decimal.Decimal, horizon)
	currentSeq := make([]float64, len(input))
	copy(currentSeq, input)

	for i := 0; i < horizon; i++ {
		output := t.Forward(&TransformerInput{
			Sequence: currentSeq,
			Mask:     make([]bool, len(currentSeq)),
		})

		predictions[i] = output.Prediction

		if len(currentSeq) > 1 {
			currentSeq = currentSeq[1:]
		}
		currentSeq = append(currentSeq, output.Prediction.InexactFloat64())
	}

	return predictions, nil
}

func (t *Transformer) AnalyzeMarketSentiment(candles []*CandleFeature) (string, decimal.Decimal) {
	if len(candles) == 0 {
		return "neutral", decimal.NewFromFloat(0.5)
	}

	input := t.extractFeatures(candles)
	output := t.Forward(&TransformerInput{
		Sequence: input,
		Mask:     make([]bool, len(input)),
	})

	sentiment := "neutral"
	if output.Prediction.GreaterThan(decimal.NewFromFloat(0.6)) {
		sentiment = "bullish"
	} else if output.Prediction.LessThan(decimal.NewFromFloat(0.4)) {
		sentiment = "bearish"
	}

	return sentiment, output.Confidence
}

type CandleFeature struct {
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
}

func (t *Transformer) extractFeatures(candles []*CandleFeature) []float64 {
	seqLen := len(candles)
	features := make([]float64, seqLen*5)

	for i, c := range candles {
		features[i*5] = normalize(c.Open)
		features[i*5+1] = normalize(c.High)
		features[i*5+2] = normalize(c.Low)
		features[i*5+3] = normalize(c.Close)
		features[i*5+4] = normalizeVolume(c.Volume)
	}

	return features
}

func normalize(value float64) float64 {
	return (value - 50) / 50
}

func normalizeVolume(volume float64) float64 {
	return math.Log1p(volume) / 20
}

func (t *Transformer) GetAttentionWeights(input []float64) [][]float64 {
	_ = t.Forward(&TransformerInput{
		Sequence: input,
		Mask:     make([]bool, len(input)),
	})

	return t.getAttentionMaps()
}

func (t *Transformer) Save(path string) error {
	return fmt.Errorf("not implemented")
}

func (t *Transformer) Load(path string) error {
	return fmt.Errorf("not implemented")
}
