package types

// HistoryStore maintains history buffers for multiple named metrics.
type HistoryStore struct {
	Buffers map[string][]float64
}

// NewHistoryStore creates a new HistoryStore.
func NewHistoryStore() *HistoryStore {
	return &HistoryStore{
		Buffers: make(map[string][]float64),
	}
}

// Push appends a value to a named buffer, keeping at most width entries.
func (s *HistoryStore) Push(name string, value float64, width int) {
	if s.Buffers == nil {
		s.Buffers = make(map[string][]float64)
	}
	buf := s.Buffers[name]
	if width <= 0 {
		return
	}
	buf = append(buf, value)
	if len(buf) > width {
		buf = buf[len(buf)-width:]
	}
	s.Buffers[name] = buf
}

// Resize trims or pads all named buffers to exactly width entries.
func (s *HistoryStore) Resize(width int) {
	if s.Buffers == nil || width <= 0 {
		return
	}
	for name, buf := range s.Buffers {
		// trim
		if len(buf) > width {
			buf = buf[len(buf)-width:]
		}
		// pad with last value if growing
		if len(buf) < width {
			padVal := 0.0
			if len(buf) > 0 {
				padVal = buf[len(buf)-1]
			}
			newBuf := make([]float64, width)
			copy(newBuf[width-len(buf):], buf)
			for i := 0; i < width-len(buf); i++ {
				newBuf[i] = padVal
			}
			buf = newBuf
		}
		s.Buffers[name] = buf
	}
}

// Get returns the named buffer.
func (s *HistoryStore) Get(name string) []float64 {
	if s.Buffers == nil {
		return nil
	}
	return s.Buffers[name]
}
