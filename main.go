package main
import(
	"fmt"
	"io"
	"net/http"
	"os"
	"crypto/sha256"
	"path/filepath"
	"encoding/hex"
)
//Store holds blob data in the memory, protected by a mutex.
// Go maps are NOT safe for concurrent access - if one goroutine writes
// while another reads, thats a racearound condition, the runtime will actually crash the program to tell you, if you run this with go run -race
//Each object key is hashed (SHA-256) into a safe hex filename, so arbitrary
//Client keys can't cause path-traversal
type Store struct{
	baseDir string // directory where all object files live
}

func NewStore(baseDir string) (*Store,error){
	//MkdirAll creates a basedirectory and a parent directory if its missing; if it exists it does nothing 
	if err :=os.MkdirAll(baseDir, 0755); err!=nil{
		return nil, err
	}
	return &Store{baseDir: baseDir}, nil
}

//Pathfor turns an object key into its in-disk file path via SHA-256.
func (s *Store) pathFor(key string) string {
	hash := sha256.Sum256([]byte(key))
	filename := hex.EncodeToString(hash[:])
	return filepath.Join(s.baseDir, filename)
}

func(s *Store) Put(key string, value []byte)error{
	return os.WriteFile(s.pathFor(key), value, 0644)
}

func (s *Store) Get(key string) ([]byte, error){
	return os.ReadFile(s.pathFor(key))
}

func (s *Store) Delete(key string)error{
	return os.Remove(s.pathFor(key))
}

func handlePut(store *Store) http.HandlerFunc{
	return func(w http.ResponseWriter, r *http.Request){
		key := r.PathValue("key")
		body, err := io.ReadAll(r.Body)
		if err!= nil{
			http.Error(w, "failed to read the body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		if err := store.Put(key, body); err!=nil{
			http.Error(w, "failed to store object", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, "Stored %q (%d bytes)\n", key, len(body))
	}
}

func handleGet(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r*http.Request){
		key := r.PathValue("key")
		value, err := store.Get(key)
		if err != nil{
			if os.IsNotExist(err){
				http.Error(w, "Object not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to read object", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(value)
	}
}

func handleDelete(store *Store) http.HandlerFunc{
	return func(w http.ResponseWriter, r*http.Request){
		key := r.PathValue("key")
		if err := store.Delete(key); err != nil{
			if os.IsNotExist(err){
				http.Error(w, "object not found", http.StatusNotFound)
				return
			}
			http.Error(w, "failed to delete object", http.StatusInternalServerError)
			return
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
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /objects/{key}", handlePut(store))
	mux.HandleFunc("GET /objects/{key}", handleGet(store))
	mux.HandleFunc("DELETE /objects/{key}", handleDelete(store))
	fmt.Println("listening in :8080")
	if err := http.ListenAndServe(":8080",mux); err!= nil{
		fmt.Println("Server error:", err)
	}
}


