package main
import(
	"fmt"
	"time"
)
// Flusher periodically drains buffered access counts from redis onto postgres
type Flusher struct{
	cache 	*Cache
	meta 	*MetadataStore
	interval time.Duration
}
func NewFlusher(cache *Cache, meta *MetadataStore, interval time.Duration) *Flusher {
	return &Flusher{cache:cache, meta: meta, interval: interval}
}

// Run loops forever, flushing every interval. Launch with: go flusher.Run()
func (f *Flusher) Run(){
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()
	fmt.Println("Flusher started: flushing access counts every %s\n", f.interval)
	for range ticker.C{
		f.flush()
	}
}

// flush drains Redis and Applies the counts to Postgres
func (f *Flusher) flush(){
	counts, err := f.cache.DrainAccessCounts()
	if err != nil{
		fmt.Println("flusher: failed to drain Redis:", err)
		return
	}
	if len(counts)==0{
		return // nothing to flush
	}
	if err := f.meta.ApplyAccessCounts(counts); err != nil{
		fmt.Println("flusher: failed to apply counts to Postgres:", err)
		return
	}
	fmt.Printf("flusher: flushed %d access counts to Postgres\n", len(counts))
}