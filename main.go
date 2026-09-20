package main
import(
	"fmt"
	"io"
	"net/http"
	"os"
	"crypto/sha256"
	"path/filepath"
	"encoding/hex"
	"time"
)
//Store holds blob data in the memory, protected by a mutex.
// Go maps are NOT safe for concurrent access - if one goroutine writes
// while another reads, thats a racearound condition, the runtime will actually crash the program to tell you, if you run this with go run -race
//Each object key is hashed (SHA-256) into a safe hex filename, so arbitrary
//Client keys can't cause path-traversal
type Store struct{
	hotDir string 
	coldDir string
}

func NewStore(baseDir string) (*Store,error){
	//MkdirAll creates a basedirectory and a parent directory if its missing; if it exists it does nothing 
	hotDir := filepath.Join(baseDir, "hot")
	coldDir := filepath.Join(baseDir, "cold")
	if err := os.MkdirAll(hotDir, 0755); err != nil {
		return nil, err
	}
	if err :=os.MkdirAll(coldDir, 0755); err != nil {
		return nil, err
	}
	return &Store{hotDir: hotDir, coldDir: coldDir}, nil

}
//dirFor returns the directory for the given tier
func (s *Store) dirFor(tier Tier) string{
	if(tier == TierCold){
		return s.coldDir
	}
	return s.hotDir
}

//pathFor turns a key + tier into the on-disk file path.
func (s *Store) pathFor(key string, tier Tier) string {
	hash := sha256.Sum256([]byte(key))
	filename := hex.EncodeToString(hash[:])
	return filepath.Join(s.dirFor(tier), filename)
}

func(s *Store) Put(key string, value []byte)error{
	return os.WriteFile(s.pathFor(key, TierHot), value, 0644)
}

func (s *Store) Get(key string, tier Tier) ([]byte, error){
	return os.ReadFile(s.pathFor(key, tier))
}

func (s *Store) Delete(key string, tier Tier)error{
	return os.Remove(s.pathFor(key, tier))
}
// Migrate moves an object's bytes from hot to cold on disk.
// Copy then delete: if we crash mid-way, the object is still readable from hot.
func (s *Store) Migrate(key string) error{
	data, err := os.ReadFile(s.pathFor(key, TierHot))
	if err != nil{
		return err
	}
	if err := os.WriteFile(s.pathFor(key, TierCold), data, 0644); err!= nil{
		return err
	}
	return os.Remove(s.pathFor(key, TierHot))
}

func handlePut(store *Store, meta *MetadataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read the body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		if err := store.Put(key, body); err != nil {
			http.Error(w, "failed to store object", http.StatusInternalServerError)
			return
		}
		if err := meta.RecordPut(key, int64(len(body))); err != nil {
			http.Error(w, "Failed to record metadata", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, "Stored %q (%d bytes)\n", key, len(body))
	}
}

func handleGet(store *Store, meta *MetadataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r*http.Request){
		key := r.PathValue("key")
		tier, err := meta.GetTier(key)
		if err != nil{
			http.Error(w, "Object not found", http.StatusNotFound)
			return 
		}
		value, err := store.Get(key, tier)
		if err != nil {
			if os.IsNotExist(err){
				http.Error(w, "Object not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to read object", http.StatusInternalServerError)
			return
		}
		if err := meta.RecordAccess(key); err!= nil{
			fmt.Println("Warning: failed to record access for", key, ":", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write(value)
	}
}


func handleDelete(store *Store, meta *MetadataStore) http.HandlerFunc{
	return func(w http.ResponseWriter, r*http.Request){
		key := r.PathValue("key")
		tier, err := meta.GetTier(key)
		if err != nil{
			http.Error(w, "Object not found", http.StatusNotFound)
			return
		}
		if err := store.Delete(key, tier); err!= nil{
			if os.IsNotExist(err){
				http.Error(w, "failed to delete object", http.StatusInternalServerError)
				return
			}
			
		}
		if err := meta.RecordDelete(key); err!=nil{
			fmt.Println("Warning; failed to delete metadata for", key, ":",err)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}


func main(){
	store, err := NewStore("./data")
	if err != nil{
		fmt.Println("Failed to init store: ", err)
		return
	}
	dsn := "postgres://blob:blob@localhost:5432/blob_storage?sslmode=disable"
	meta, err := NewMetadataStore(dsn)
	if err != nil{
		fmt.Println("Failed to init metadata store: ", err)
		return
	}
	defer meta.Close()
	fmt.Println("connected to Postgres, schema ready")
	mover := NewMover(store, meta, 30*time.Second, 10*time.Second)
	go mover.Run()
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /objects/{key}", handlePut(store, meta))
	mux.HandleFunc("GET /objects/{key}", handleGet(store, meta))
	mux.HandleFunc("DELETE /objects/{key}", handleDelete(store, meta))
	fmt.Println("listening in :8080")
	if err := http.ListenAndServe(":8080",mux); err!= nil{
		fmt.Println("Server error:", err)
	}
}


