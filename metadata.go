package main
import(
	"database/sql"
	"time"

	_"github.com/jackc/pgx/v5/stdlib"
)
// Tier is the storage tier an object currently lives in
type Tier string

const(
	TierHot Tier = "hot"
	TierCold Tier = "cold"
)

//ObjectMeta mirrors one row of the objects table - the facts we track
// about an object (never its contents)

type ObjectMeta struct {
	Key 	string
	Size 	int64
	Tier 	Tier
	CreatedAt 	time.Time
	LastAccessed 	time.Time
	AccessCount 	int64
}

// MetadataStore wraps Postgres connection andd all metadata operations
type MetadataStore struct {
	db*sql.DB
}

// NewMetadataStore connects to Postgres and ensures the schema exists
func NewMetadataStore(dsn string) (*MetadataStore, error){
	db, err := sql.Open("pgx", dsn)
	if err!= nil{
		return nil, err
	}
	if err := db.Ping(); err!= nil{
		return nil, err
	}
	store := &MetadataStore{db: db}
	if err := store.migrate(); err!= nil{
		return nil, err
	}
	return store, nil
}
// migrate creates the objects table if it doesnt already exist
// safe to run on every startup (idempotent)

func (m *MetadataStore) migrate() error{
	const schema = `CREATE TABLE IF NOT EXISTS objects(
	key 				TEXT PRIMARY KEY,
	size 				BIGINT NOT NULL,
	tier 				TEXT NOT NULL DEFAULT 'hot',
	created_at  		TIMESTAMPTZ NOT NULL DEFAULT now(),
	last_accessed 		TIMESTAMPTZ NOT NULL DEFAULT now(),
	access_count 		BIGINT NOT NULL DEFAULT 0
	);`
	

_, err := m.db.Exec(schema)
return err
}
//RecordPut inserts or updates an object's metadata after it's sorted.
func (m *MetadataStore) RecordPut(key string, size int64) error{
	const q = `
	INSERT INTO objects (key, size, tier, last_accessed)
	VALUES ($1, $2, 'hot', now())
	ON CONFLICT (key) DO UPDATE
	SET size = EXCLUDED.size, last_accessed = now();`
	_, err := m.db.Exec(q, key, size)
	return err
}
//RecordDelete removes an object's metadata row.
func (m*MetadataStore) RecordDelete(key string) error{
	const q = `DELETE FROM objects WHERE key = $1;`
	_, err := m.db.Exec(q, key)
	return err
}
//RecordAccess bumps access_count and last_accesed on every GET.
func (m*MetadataStore) RecordAccess(key string) error{
	const q = `UPDATE objects
	SET access_count = access_count + 1, last_accessed = now()
	WHERE key = $1;`
	_, err := m.db.Exec(q, key)
	return err
}
// Close releases the connection pool
func (m *MetadataStore) Close()error{
	return m.db.Close()
}

