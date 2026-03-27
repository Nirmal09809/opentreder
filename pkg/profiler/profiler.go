package profiler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opentreder/opentreder/pkg/logger"
)

type Profiler struct {
	mu           sync.RWMutex
	config       Config
	enabled      atomic.Bool
	cpuProfile   *pprof.Profile
	memProfile   *pprof.Profile
	metrics      *Metrics
	counters     map[string]*Counter
	gauges       map[string]*Gauge
	histograms   map[string]*Histogram
	timers       map[string]*Timer
	startTime    time.Time
	ctx          context.Context
	cancel       context.CancelFunc
}

type Config struct {
	Enabled       bool          `json:"enabled"`
	CPUProfile    bool          `json:"cpu_profile"`
	MemoryProfile bool          `json:"memory_profile"`
	BlockProfile  bool          `json:"block_profile"`
	MutexProfile  bool          `json:"mutex_profile"`
	ProfilePath   string        `json:"profile_path"`
	ReportPath    string        `json:"report_path"`
	Interval      time.Duration `json:"interval"`
	Percentiles   []float64     `json:"percentiles"`
}

type Metrics struct {
	Counters   map[string]CounterValue   `json:"counters"`
	Gauges     map[string]float64       `json:"gauges"`
	Histograms map[string]HistogramData  `json:"histograms"`
	Timers     map[string]TimerData     `json:"timers"`
	Runtime    RuntimeMetrics          `json:"runtime"`
}

type CounterValue struct {
	Value  int64   `json:"value"`
	Rate   float64 `json:"rate"`
	Total  int64   `json:"total"`
}

type GaugeValue struct {
	Value float64 `json:"value"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

type HistogramData struct {
	Count    int64     `json:"count"`
	Sum      float64   `json:"sum"`
	Min      float64   `json:"min"`
	Max      float64   `json:"max"`
	Mean     float64   `json:"mean"`
	StdDev   float64   `json:"std_dev"`
	P50      float64   `json:"p50"`
	P75      float64   `json:"p75"`
	P90      float64   `json:"p90"`
	P95      float64   `json:"p95"`
	P99      float64   `json:"p99"`
	P999     float64   `json:"p999"`
}

type TimerData struct {
	Count     int64     `json:"count"`
	Sum       float64   `json:"sum"`
	Min       float64   `json:"min"`
	Max       float64   `json:"max"`
	Mean      float64   `json:"mean"`
	StdDev    float64   `json:"std_dev"`
	Rate      float64   `json:"rate"`
	P50       float64   `json:"p50"`
	P75       float64   `json:"p75"`
	P90       float64   `json:"p90"`
	P95       float64   `json:"p95"`
	P99       float64   `json:"p99"`
}

type RuntimeMetrics struct {
	GoRoutines     int     `json:"goroutines"`
	MemoryAlloc    uint64  `json:"memory_alloc_bytes"`
	MemoryTotal    uint64  `json:"memory_total_bytes"`
	MemorySys      uint64  `json:"memory_sys_bytes"`
	GCCount        uint32  `json:"gc_count"`
	GCPauseTotal   float64 `json:"gc_pause_total_ms"`
	CGoCalls       int64   `json:"cgo_calls"`
	CPUCount       int     `json:"cpu_count"`
	NumThread      int     `json:"num_thread"`
}

type Counter struct {
	name   string
	value  int64
	total  int64
	rate   float64
	mu     sync.RWMutex
}

type Gauge struct {
	name  string
	value float64
	min   float64
	max   float64
	mu    sync.RWMutex
}

type Histogram struct {
	name       string
	values    []float64
	count     int64
	sum       float64
	mu        sync.RWMutex
	percentiles []float64
}

type Timer struct {
	name       string
	count      int64
	sum        float64
	min        float64
	max        float64
	mean       float64
	m2         float64
	lastUpdate time.Time
	mu         sync.RWMutex
}

type LatencyTracker struct {
	mu           sync.RWMutex
	windowSize   time.Duration
	windowStart  time.Time
	windowData   []time.Duration
	currentIndex int
}

type ProfileResult struct {
	Duration   time.Duration      `json:"duration"`
	StartTime  time.Time          `json:"start_time"`
	EndTime    time.Time          `json:"end_time"`
	Metrics    *Metrics           `json:"metrics"`
	CPUProfile *CPUProfileData    `json:"cpu_profile,omitempty"`
	MemProfile *MemProfileData    `json:"mem_profile,omitempty"`
	Analysis   *AnalysisResult    `json:"analysis"`
}

type CPUProfileData struct {
	Duration time.Duration `json:"duration"`
	TopFunctions []FunctionProfile `json:"top_functions"`
}

type MemProfileData struct {
	AllocBytes    uint64  `json:"alloc_bytes"`
	TotalAlloc    uint64  `json:"total_alloc_bytes"`
	Objects       uint64  `json:"objects"`
	TopAllocations []AllocationProfile `json:"top_allocations"`
}

type FunctionProfile struct {
	Name       string  `json:"name"`
	PkgPath    string  `json:"pkg_path"`
	PercentCPU float64 `json:"percent_cpu"`
	PercentMem float64 `json:"percent_mem"`
	FlatCPU    float64 `json:"flat_cpu_ms"`
	FlatMem    float64 `json:"flat_mem_kb"`
	CumCPU     float64 `json:"cum_cpu_ms"`
	CumMem     float64 `json:"cum_mem_kb"`
	Count      int     `json:"count"`
}

type AllocationProfile struct {
	Name       string `json:"name"`
	PkgPath    string `json:"pkg_path"`
	AllocBytes uint64 `json:"alloc_bytes"`
	AllocObjects uint64 `json:"alloc_objects"`
	PercentAlloc float64 `json:"percent_alloc"`
}

type AnalysisResult struct {
	SlowFunctions    []string  `json:"slow_functions"`
	MemoryHogs       []string  `json:"memory_hogs"`
	Recommendations  []string  `json:"recommendations"`
	Bottlenecks      []string  `json:"bottlenecks"`
	Optimizations    []string  `json:"optimizations"`
	Score            int       `json:"score"`
	Grade            string    `json:"grade"`
}

var defaultPercentiles = []float64{0.5, 0.75, 0.9, 0.95, 0.99, 0.999}

func NewProfiler(cfg Config) *Profiler {
	if len(cfg.Percentiles) == 0 {
		cfg.Percentiles = defaultPercentiles
	}
	if cfg.Interval == 0 {
		cfg.Interval = 15 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Profiler{
		config:     cfg,
		metrics:    &Metrics{},
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
		timers:     make(map[string]*Timer),
		startTime:   time.Now(),
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (p *Profiler) Start() error {
	if !p.config.Enabled {
		return nil
	}

	p.enabled.Store(true)

	if p.config.CPUProfile {
		if err := p.startCPUProfile(); err != nil {
			return fmt.Errorf("failed to start CPU profile: %w", err)
		}
	}

	if p.config.MemoryProfile {
		if err := p.startMemoryProfile(); err != nil {
			return fmt.Errorf("failed to start memory profile: %w", err)
		}
	}

	go p.collectMetrics()
	go p.generateReport()

	logger.Info("Profiler started", 
		"cpu_profile", p.config.CPUProfile,
		"memory_profile", p.config.MemoryProfile,
	)

	return nil
}

func (p *Profiler) Stop() error {
	if !p.enabled.Load() {
		return nil
	}

	p.cancel()
	p.enabled.Store(false)

	var errs []error

	if p.cpuProfile != nil {
		pprof.StopCPUProfile()
	}

	if err := p.saveProfiles(); err != nil {
		errs = append(errs, err)
	}

	logger.Info("Profiler stopped", "duration", time.Since(p.startTime))

	if len(errs) > 0 {
		return fmt.Errorf("profiler errors: %v", errs)
	}
	return nil
}

func (p *Profiler) startCPUProfile() error {
	profilePath := p.config.ProfilePath + "/cpu.prof"
	f, err := os.Create(profilePath)
	if err != nil {
		return err
	}

	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return err
	}

	p.cpuProfile = pprof.Lookup("cpu")
	return nil
}

func (p *Profiler) startMemoryProfile() error {
	debug.SetGCPercent(100)
	runtime.MemProfileRate = 4096

	if p.config.BlockProfile {
		runtime.SetBlockProfileRate(1)
	}

	if p.config.MutexProfile {
		runtime.SetMutexProfileFraction(1)
	}

	p.memProfile = pprof.Lookup("heap")
	return nil
}

func (p *Profiler) collectMetrics() {
	ticker := time.NewTicker(p.config.Interval)
	defer ticker.Stop()

	var m runtime.MemStats
	var lastGC uint32

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.mu.Lock()

			runtime.ReadMemStats(&m)

			p.metrics.Runtime = RuntimeMetrics{
				GoRoutines:   runtime.NumGoroutine(),
				MemoryAlloc: m.Alloc,
				MemoryTotal: m.TotalAlloc,
				MemorySys:   m.Sys,
				GCCount:     m.NumGC,
				GCPauseTotal: float64(m.PauseTotalNs) / 1e6,
				CGoCalls:    runtime.NumCgoCall(),
				CPUCount:    runtime.NumCPU(),
				NumThread:   threadCreateCount(),
			}

			if m.NumGC > lastGC {
				lastGC = m.NumGC
			}

			p.mu.Unlock()
		}
	}
}

func (p *Profiler) generateReport() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			result := p.GenerateReport()
			if result != nil {
				p.saveReport(result)
			}
		}
	}
}

func (p *Profiler) GenerateReport() *ProfileResult {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := &ProfileResult{
		Duration:  time.Since(p.startTime),
		StartTime: p.startTime,
		EndTime:   time.Now(),
		Metrics:   p.metrics,
	}

	result.Analysis = p.analyze()

	return result
}

func (p *Profiler) analyze() *AnalysisResult {
	analysis := &AnalysisResult{
		SlowFunctions:   make([]string, 0),
		MemoryHogs:     make([]string, 0),
		Recommendations: make([]string, 0),
		Bottlenecks:    make([]string, 0),
		Optimizations:  make([]string, 0),
	}

	if p.metrics.Runtime.GoRoutines > 1000 {
		analysis.Recommendations = append(analysis.Recommendations, 
			"High number of goroutines detected. Consider pooling or limiting concurrency.")
	}

	if p.metrics.Runtime.MemoryAlloc > 1e9 {
		analysis.MemoryHogs = append(analysis.MemoryHogs, "Memory usage exceeds 1GB")
		analysis.Recommendations = append(analysis.Recommendations,
			"Consider reducing memory allocations or implementing object pooling.")
	}

	if p.metrics.Runtime.GCPauseTotal > 100 {
		analysis.Bottlenecks = append(analysis.Bottlenecks, "High GC pause time")
		analysis.Optimizations = append(analysis.Optimizations,
			"Reduce allocations in hot paths, use sync.Pool for temporary objects.")
	}

	analysis.Score = calculateScore(analysis)
	analysis.Grade = getGrade(analysis.Score)

	return analysis
}

func calculateScore(a *AnalysisResult) int {
	score := 100

	if len(a.SlowFunctions) > 5 {
		score -= 20
	}
	if len(a.MemoryHogs) > 0 {
		score -= 15 * len(a.MemoryHogs)
	}
	if len(a.Bottlenecks) > 0 {
		score -= 10 * len(a.Bottlenecks)
	}

	if score < 0 {
		score = 0
	}
	return score
}

func getGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

func (p *Profiler) saveProfiles() error {
	if p.cpuProfile != nil {
		profilePath := p.config.ProfilePath + "/cpu_final.prof"
		f, err := os.Create(profilePath)
		if err != nil {
			return err
		}
		defer f.Close()

		pprof.Lookup("heap").WriteTo(f, 0)
	}

	return nil
}

func (p *Profiler) saveReport(result *ProfileResult) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal profile report", "error", err)
		return
	}

	reportPath := fmt.Sprintf("%s/report_%s.json", 
		p.config.ReportPath, 
		time.Now().Format("20060102_150405"))
	
	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		logger.Error("Failed to save profile report", "error", err)
		return
	}

	logger.Info("Profile report saved", "path", reportPath)
}

func (p *Profiler) NewCounter(name string) *Counter {
	p.mu.Lock()
	defer p.mu.Unlock()

	counter := &Counter{name: name}
	p.counters[name] = counter
	return counter
}

func (p *Profiler) NewGauge(name string) *Gauge {
	p.mu.Lock()
	defer p.mu.Unlock()

	gauge := &Gauge{name: name, min: float64(^uint64(0) >> 1), max: -float64(^uint64(0) >> 1)}
	p.gauges[name] = gauge
	return gauge
}

func (p *Profiler) NewHistogram(name string) *Histogram {
	p.mu.Lock()
	defer p.mu.Unlock()

	hist := &Histogram{
		name:        name,
		values:      make([]float64, 0, 10000),
		percentiles: p.config.Percentiles,
	}
	p.histograms[name] = hist
	return hist
}

func (p *Profiler) NewTimer(name string) *Timer {
	p.mu.Lock()
	defer p.mu.Unlock()

	timer := &Timer{name: name, min: float64(^uint64(0) >> 1), max: -1}
	p.timers[name] = timer
	return timer
}

func (p *Profiler) IncCounter(name string) {
	if counter, ok := p.counters[name]; ok {
		counter.Inc()
	} else {
		p.NewCounter(name).Inc()
	}
}

func (p *Profiler) SetGauge(name string, value float64) {
	if gauge, ok := p.gauges[name]; ok {
		gauge.Set(value)
	} else {
		p.NewGauge(name).Set(value)
	}
}

func (p *Profiler) RecordHistogram(name string, value float64) {
	if hist, ok := p.histograms[name]; ok {
		hist.Record(value)
	} else {
		p.NewHistogram(name).Record(value)
	}
}

func (p *Profiler) RecordTimer(name string, duration time.Duration) {
	d := float64(duration.Microseconds()) / 1000
	if timer, ok := p.timers[name]; ok {
		timer.Record(d)
	} else {
		p.NewTimer(name).Record(d)
	}
}

func (p *Profiler) GetMetrics() *Metrics {
	p.mu.RLock()
	defer p.mu.RUnlock()

	m := &Metrics{
		Counters:   make(map[string]CounterValue),
		Gauges:     make(map[string]float64),
		Histograms: make(map[string]HistogramData),
		Timers:     make(map[string]TimerData),
	}

	for name, c := range p.counters {
		m.Counters[name] = c.Value()
	}

	for name, g := range p.gauges {
		m.Gauges[name] = g.Value()
	}

	for name, h := range p.histograms {
		m.Histograms[name] = h.Data()
	}

	for name, t := range p.timers {
		m.Timers[name] = t.Data()
	}

	return m
}

func (c *Counter) Inc() {
	c.mu.Lock()
	c.value++
	c.total++
	c.mu.Unlock()
}

func (c *Counter) Add(v int64) {
	c.mu.Lock()
	c.value += v
	c.total += v
	c.mu.Unlock()
}

func (c *Counter) Value() CounterValue {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CounterValue{
		Value: c.value,
		Total: c.total,
		Rate:  c.rate,
	}
}

func (g *Gauge) Set(value float64) {
	g.mu.Lock()
	g.value = value
	if value < g.min {
		g.min = value
	}
	if value > g.max {
		g.max = value
	}
	g.mu.Unlock()
}

func (g *Gauge) Value() float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.value
}

func (h *Histogram) Record(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.values = append(h.values, value)
	h.count++
	h.sum += value

	if len(h.values) > 10000 {
		h.values = h.values[len(h.values)-10000:]
	}
}

func (h *Histogram) Data() HistogramData {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data := HistogramData{
		Count: h.count,
		Sum:   h.sum,
	}

	if len(h.values) == 0 {
		return data
	}

	values := make([]float64, len(h.values))
	copy(values, h.values)
	quickSort(values, 0, len(values)-1)

	data.Min = values[0]
	data.Max = values[len(values)-1]
	data.Mean = data.Sum / float64(len(values))

	var variance float64
	for _, v := range values {
		diff := v - data.Mean
		variance += diff * diff
	}
	data.StdDev = variance / float64(len(values))

	for i, p := range h.percentiles {
		idx := int(float64(len(values)) * p)
		if idx >= len(values) {
			idx = len(values) - 1
		}
		switch i {
		case 0:
			data.P50 = values[idx]
		case 1:
			data.P75 = values[idx]
		case 2:
			data.P90 = values[idx]
		case 3:
			data.P95 = values[idx]
		case 4:
			data.P99 = values[idx]
		case 5:
			data.P999 = values[idx]
		}
	}

	return data
}

func (t *Timer) Record(value float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.count++
	t.sum += value

	if value < t.min {
		t.min = value
	}
	if value > t.max {
		t.max = value
	}

	delta := value - t.mean
	t.mean += delta / float64(t.count)
	delta2 := value - t.mean
	t.m2 += delta * delta2

	t.lastUpdate = time.Now()
}

func (t *Timer) Data() TimerData {
	t.mu.RLock()
	defer t.mu.RUnlock()

	data := TimerData{
		Count:  t.count,
		Sum:    t.sum,
		Min:    t.min,
		Max:    t.max,
		Mean:   t.mean,
		Rate:   float64(t.count) / time.Since(t.lastUpdate).Seconds(),
	}

	if t.count > 1 {
		data.StdDev = t.m2 / float64(t.count-1)
	}

	return data
}

func quickSort(a []float64, lo, hi int) {
	if lo < hi {
		p := partition(a, lo, hi)
		quickSort(a, lo, p-1)
		quickSort(a, p+1, hi)
	}
}

func partition(a []float64, lo, hi int) int {
	pivot := a[hi]
	i := lo - 1
	for j := lo; j < hi; j++ {
		if a[j] < pivot {
			i++
			a[i], a[j] = a[j], a[i]
		}
	}
	a[i+1], a[hi] = a[hi], a[i+1]
	return i + 1
}

func threadCreateCount() int {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int(m.Mallocs)
}

type Trace struct {
	name      string
	startTime time.Time
	tags      map[string]string
}

func StartTrace(name string) *Trace {
	return &Trace{
		name:      name,
		startTime: time.Now(),
		tags:      make(map[string]string),
	}
}

func (t *Trace) WithTag(key, value string) *Trace {
	t.tags[key] = value
	return t
}

func (t *Trace) End() time.Duration {
	duration := time.Since(t.startTime)
	return duration
}

func (t *Trace) EndWithRecord(profiler *Profiler) time.Duration {
	duration := t.End()
	profiler.RecordTimer(t.name, duration)
	return duration
}
