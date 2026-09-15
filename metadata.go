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

// Close releases the connection pool
func (m *MetadataStore) Close()error{
	return m.db.Close()
}

