package metrics

type Map struct {
	clients map[string]*Bucket
}

func newMap() *Map {
	return &Map{
		clients: make(map[string]*Bucket),
	}
}

func (m *Map) AddMetric(key, prefix string) {
	bucket, _ := NewBucket(prefix)
	m.clients[key] = bucket
}

func (m *Map) GetMetric(key string) (*Bucket, bool) {
	client, ok := m.clients[key]
	return client, ok
}
