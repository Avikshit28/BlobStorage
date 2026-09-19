package main
import (
	"fmt"
	"time"
)
//Mover periodically migrates cold candidates from hot to cold storage
type Mover struct{
	store   *Store
	meta 	*MetadataStore
	coldAfter 	time.Duration // migrate objects idle longer than this 
	interval 	time.Duration // how often to check
}
func NewMover(store *Store, meta *MetadataStore, coldAfter, interval time.Duration) *Mover{
	return &Mover{
		store: 	store,
		meta: 	meta,
		coldAfter:	coldAfter,
		interval: 	interval,
	}
}
//Run loops forever, checking for migration candidates every `interval`
// Intended to be launched in a goroutine: `go mover.Run()`
func (mv *Mover) Run(){
	ticker := time.NewTicker(mv.interval)
	defer ticker.Stop()
	fmt.Println("mover started: cold after %s, checking every %s\n", mv.coldAfter, mv.interval)
	for range ticker.C{
		mv.sweep()
	}

}
//sweep runs one migration pass
func (mv *Mover) sweep(){
	cutoff := time.Now().Add(-mv.coldAfter)
	keys, err := mv.meta.MigrationCandidates(cutoff)
	if err != nil{
		fmt.Println("mover: failed to find Candidates:", err)
		return
	}
	for _, key := range keys {
		if err := mv.migrateOne(key); err!= nil{
			fmt.Printf("mover: failed to migrate %q: %v\n", key, err)
		}
	}
}
//migrateOne moves a single object hot ->cold: bytes first, then the tier flag
func (mv *Mover) migrateOne(key string) error{
	if err := mv.store.Migrate(key); err!= nil{
		return err
	}
	if err := mv.meta.SetTier(key, TierCold); err!= nil{
		return err
	}
	fmt.Printf("mover: migrated %q hot -> cold\n", key)
	return nil
}